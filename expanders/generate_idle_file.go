package expanders

import (
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"log"
	"os"
	"path"
	"sync"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	DEFAULT_IDLE_FILES_PATH = "./Proofs"
	LAYER_NAME              = "layer"
	COMMIT_FILE             = "roots"
	IDLE_DIR_NAME           = "idlefile"
)

var (
	HashSize = 64
	pool     *sync.Pool
)

type Pool interface {
	Get() any
	Put(any)
}

func GetPool() Pool {
	return pool
}

func MakeProofDir(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return os.MkdirAll(dir, 0777)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0777)
}

func NewHash() hash.Hash {
	switch HashSize {
	case 32:
		return sha256.New()
	case 64:
		return sha512.New()
	default:
		return sha512.New()
	}
}

func GetHash(data []byte) []byte {
	h := NewHash()
	if data == nil {
		data = []byte("none")
	}
	h.Write(data)
	return h.Sum(nil)
}

// InitLabelsPool init pool of []byte for caching node labels,
// n is node numbers of one layer of expanders,size is hash func output size
func InitLabelsPool(n, size int64) {
	//init pool if it is nil
	if pool == nil {
		pool = &sync.Pool{
			New: func() any {
				buf := make([]byte, n*size)
				return &buf
			},
		}
	}
}

func (expanders *Expanders) GenerateIdleFile(minerID []byte, Count int64, rootDir string) error {
	//generate tmp dir name
	dir := path.Join(rootDir, fmt.Sprintf("%s-%d", IDLE_DIR_NAME, Count))
	if err := MakeProofDir(dir); err != nil {
		errors.Wrap(err, "generate idle file error")
	}

	ch := expanders.RunRelationalMapServer(minerID, Count)
	hash := NewHash()

	//create aux slices
	roots := make([][]byte, expanders.K+2)
	parents := pool.Get().(*[]byte)
	labels := pool.Get().(*[]byte)

	labelLeftSize := len(minerID) + int(unsafe.Sizeof(NodeType(0))) + 8
	label := make([]byte, labelLeftSize+int(expanders.D+1)*HashSize)
	//calculate labels layer by layer
	for i := int64(0); i <= expanders.K; i++ {
		for j := int64(0); j < expanders.N; j++ {
			node := <-ch
			util.CopyData(label, minerID,
				GetBytes(Count), GetBytes(NodeType(i*expanders.N+j)))
			bytesCount := labelLeftSize
			if i > 0 && !node.NoParents() {
				for _, idx := range node.Parents {
					if int64(idx) < expanders.N {
						l, r := idx*NodeType(HashSize), (idx+1)*NodeType(HashSize)
						copy(label[bytesCount:bytesCount+HashSize], (*parents)[l:r])
						bytesCount += HashSize
					} else {
						l, r := (int64(idx)-expanders.N)*int64(HashSize), (int64(idx)-expanders.N+1)*int64(HashSize)
						copy(label[bytesCount:bytesCount+HashSize], (*labels)[l:r])
						bytesCount += HashSize
					}
				}
			}
			hash.Reset()
			hash.Write(label)
			copy((*labels)[j*int64(HashSize):(j+1)*int64(HashSize)], hash.Sum(nil))
		}
		//calculate merkel tree root hash for each layer
		ltree := tree.CalcLightMhtWithBytes((*labels), HashSize, true)
		roots[i] = ltree.GetRoot(HashSize)
		tree.RecoveryMht(ltree)

		//save one layer labels
		if err := util.SaveFile(path.Join(dir, fmt.Sprintf("%s-%d", LAYER_NAME, i)), (*labels)); err != nil {
			return errors.Wrap(err, "generate idle file error")
		}
		parents, labels = labels, parents
	}

	pool.Put(parents)
	pool.Put(labels)
	//calculate new dir name
	hash.Reset()
	for i := 0; i < len(roots); i++ {
		hash.Write(roots[i])
	}
	roots[expanders.K+1] = hash.Sum(nil)

	if err := util.SaveProofFile(path.Join(dir, COMMIT_FILE), roots); err != nil {
		return errors.Wrap(err, "generate idle file error")
	}

	return nil
}

func IdleFileGenerationServer(expanders *Expanders, minerID []byte, rootDir string, tNum int) (chan<- int64, <-chan bool) {
	in, out := make(chan int64, tNum), make(chan bool, tNum)
	for i := 0; i < tNum; i++ {
		go func() {
			for count := range in {
				if count <= 0 {
					close(out)
					return
				}
				err := expanders.GenerateIdleFile(minerID, count, rootDir)
				if err != nil {
					log.Println(err)
				}
				out <- true
			}
		}()
	}
	return in, out
}
