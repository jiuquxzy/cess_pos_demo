package expanders

import (
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"os"
	"path"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	DEFAULT_IDLE_FILES_PATH = "./Proofs"
	LAYER_NAME              = "layer"
	COMMIT_FILE             = "roots"
)

var HashSize = 64

func MakeProofDir(rootPath, name string) error {
	dir := path.Join(rootPath, name)
	if _, err := os.Stat(dir); err == nil {
		return errors.New("dir is already exist")
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

func CreateRandDir(root string) (string, error) {
	for count := 0; count < 10; count++ {
		name := util.RandString(32)
		if err := MakeProofDir(root, name); err == nil {
			return path.Join(root, name), nil
		} else if count+1 >= 10 {
			return "", err
		}
	}
	return "", nil
}

func (expanders *Expanders) GenerateIdleFile(minerID []byte, Count int64, rootDir string) (string, error) {
	//generate tmp dir name
	dir, err := CreateRandDir(rootDir)
	if err != nil {
		return dir, errors.Wrap(err, "generate idle file error")
	}

	hash := NewHash()

	//create aux slices
	roots := make([][]byte, expanders.K+1)
	parents := make([]byte, expanders.N*int64(HashSize))
	labels := make([]byte, expanders.N*int64(HashSize))

	labelLeftSize := len(expanders.ID) + len(minerID) + int(unsafe.Sizeof(NodeType(0))) + 8
	label := make([]byte, labelLeftSize+int(expanders.D+1)*HashSize)
	//calculate labels layer by layer
	for i := int64(0); i <= expanders.K; i++ {
		rmap := *expanders.GetRelationalMap()
		for j := int64(0); j < expanders.N; j++ {
			node := rmap[j]
			copyData(label, expanders.ID, minerID,
				GetBytes(Count), GetBytes(NodeType(i*expanders.N+j)))
			bytesCount := labelLeftSize
			if i > 0 && !node.NoParents() {
				for _, idx := range node.Parents {
					if int64(idx) < expanders.N {
						l, r := idx*NodeType(HashSize), (idx+1)*NodeType(HashSize)
						copy(label[bytesCount:bytesCount+HashSize], parents[l:r])
						bytesCount += HashSize
					} else {
						l, r := (int64(idx)-expanders.N)*int64(HashSize), (int64(idx)-expanders.N+1)*int64(HashSize)
						copy(label[bytesCount:bytesCount+HashSize], labels[l:r])
						bytesCount += HashSize
					}
				}
			}
			hash.Reset()
			hash.Write(label)
			copy(labels[j*int64(HashSize):(j+1)*int64(HashSize)], hash.Sum(nil))
		}
		//calculate merkel tree root hash for each layer
		root, err := tree.CalculateMerkelTreeRoot2(labels, HashSize)
		if err != nil {
			return dir, errors.Wrap(err, "generate idle file error")
		}
		roots[i] = root
		//save one layer labels
		if err = util.SaveFile(path.Join(dir, fmt.Sprintf("%s-%d", LAYER_NAME, i)), labels); err != nil {
			return dir, errors.Wrap(err, "generate idle file error")
		}
		parents, labels = labels, parents
	}
	//calculate new dir name
	hash.Reset()
	for i := 0; i < len(roots); i++ {
		hash.Write(roots[i])
	}
	name := hex.EncodeToString(hash.Sum(nil))
	if err = os.Rename(dir, path.Join(rootDir, name)); err != nil {
		return dir, errors.Wrap(err, "generate idle file error")
	}
	dir = path.Join(rootDir, name)
	if err = util.SaveProofFile(path.Join(dir, COMMIT_FILE), roots); err != nil {
		return dir, errors.Wrap(err, "generate idle file error")
	}

	return dir, nil
}

func (expanders *Expanders) GenerateIdleFileWitRandExpanders(minerID []byte, Count int64, rootDir string) (string, error) {
	//generate tmp dir name
	dir, err := CreateRandDir(rootDir)
	if err != nil {
		return dir, errors.Wrap(err, "generate idle file error")
	}

	ch := RunRelationalMapServer(expanders, minerID, Count)
	hash := NewHash()

	//create aux slices
	roots := make([][]byte, expanders.K+1)
	parents := make([]byte, expanders.N*int64(HashSize))
	labels := make([]byte, expanders.N*int64(HashSize))

	labelLeftSize := len(expanders.ID) + len(minerID) + int(unsafe.Sizeof(NodeType(0))) + 8
	label := make([]byte, labelLeftSize+int(expanders.D+1)*HashSize)
	//calculate labels layer by layer
	for i := int64(0); i <= expanders.K; i++ {
		for j := int64(0); j < expanders.N; j++ {
			node := <-ch
			copyData(label, expanders.ID, minerID,
				GetBytes(Count), GetBytes(NodeType(i*expanders.N+j)))
			bytesCount := labelLeftSize
			if i > 0 && !node.NoParents() {
				for _, idx := range node.Parents {
					if int64(idx) < expanders.N {
						l, r := idx*NodeType(HashSize), (idx+1)*NodeType(HashSize)
						copy(label[bytesCount:bytesCount+HashSize], parents[l:r])
						bytesCount += HashSize
					} else {
						l, r := (int64(idx)-expanders.N)*int64(HashSize), (int64(idx)-expanders.N+1)*int64(HashSize)
						copy(label[bytesCount:bytesCount+HashSize], labels[l:r])
						bytesCount += HashSize
					}
				}
			}
			hash.Reset()
			hash.Write(label)
			copy(labels[j*int64(HashSize):(j+1)*int64(HashSize)], hash.Sum(nil))
		}
		//calculate merkel tree root hash for each layer
		ltree := tree.CalcLightMmtWitBytes(labels, HashSize)
		roots[i] = ltree.GetRoot(HashSize)
		//save one layer labels
		if err = util.SaveFile(path.Join(dir, fmt.Sprintf("%s-%d", LAYER_NAME, i)), labels); err != nil {
			return dir, errors.Wrap(err, "generate idle file error")
		}
		parents, labels = labels, parents
	}
	//calculate new dir name
	hash.Reset()
	for i := 0; i < len(roots); i++ {
		hash.Write(roots[i])
	}
	name := hex.EncodeToString(hash.Sum(nil))
	if err = os.Rename(dir, path.Join(rootDir, name)); err != nil {
		return dir, errors.Wrap(err, "generate idle file error")
	}
	dir = path.Join(rootDir, name)
	if err = util.SaveProofFile(path.Join(dir, COMMIT_FILE), roots); err != nil {
		return dir, errors.Wrap(err, "generate idle file error")
	}

	return dir, nil
}

func copyData(target []byte, src ...[]byte) {
	count, lens := 0, len(target)
	for _, d := range src {
		l := len(d)
		if l == 0 || l+count > lens {
			continue
		}
		count += l
		copy(target[count-l:count], d)
	}
}
