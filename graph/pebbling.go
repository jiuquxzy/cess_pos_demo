package graph

import (
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"log"
	"os"
	"path"
	"sort"

	"github.com/pkg/errors"
)

const (
	DEFAULT_PATH = "./Proofs"
	LAYER_NAME   = "layer"
	COMMIT_FILE  = "roots"
)

var HashSize = 64

func MakeProofDir(rootPath, name string) error {
	dir := path.Join(rootPath, name)
	if _, err := os.Stat(dir); err == nil {
		return errors.New("dir is already exist")
	}
	return os.MkdirAll(dir, 0777)
}

// PebblingGraph is used to simulate the process of generating node label values in pebbling game.
// Note that the calculation process will consume O(3N) of auxiliary space,
// although localization techniques are used. It use N-sized memory to accelerate calculations,
// and O(N) space for calculating MerkelTree.
func PebblingGraph(g *StackedExpanders, ID []byte, Count int64, rdir string) (string, error) {
	hash := NewHash()
	var (
		dir    string
		labels [][]byte
		err    error
	)
	//generate tmp dir name
	for {
		name := util.RandString(32)
		if err = MakeProofDir(rdir, name); err == nil {
			dir = path.Join(rdir, name)
			break
		} else {
			log.Println("make dir error", err)
		}
	}
	roots := make([][]byte, g.K+1)
	//calculate pebbling labels layer by layer
	for i := int64(0); i <= g.K; i++ {
		layerLabs := make([][]byte, g.N)
		//load parents' label
		if i > 0 {
			labels, err = util.ReadProofFile(
				path.Join(dir, fmt.Sprintf("%s-%d", LAYER_NAME, i-1)),
				int(g.N), HashSize)
			if err != nil {
				return dir, errors.Wrap(err, "pebbling graph error")
			}
		}
		//calculate each node's label
		for j := i * g.N; j < (i+1)*g.N; j++ {
			node, err := g.GetNode(j)
			if err != nil {
				return dir, errors.Wrap(err, "pebbling graph error")
			}
			label := append(ID, util.Int64Bytes(Count)...)
			label = append(label, util.Int64Bytes(node.Index)...)
			if !node.NoParents() {
				parents := Sort(node.Parents)
				for _, idx := range parents {
					if idx >= i*g.N {
						label = append(label, layerLabs[idx-i*g.N]...)
					} else {
						label = append(label, labels[idx-(i-1)*g.N]...)
					}
				}
			}
			hash.Reset()
			hash.Write(label)
			layerLabs[int(j-i*g.N)] = hash.Sum(nil)
		}
		//calculate merkel tree root hash for each layer
		root, err := tree.CalculateMerkelTreeRoot(layerLabs)
		if err != nil {
			return dir, errors.Wrap(err, "pebbling graph error")
		}
		roots[i] = root
		//save one layer labels
		if err = util.SaveProofFile(
			path.Join(dir, fmt.Sprintf("%s-%d", LAYER_NAME, i)),
			layerLabs); err != nil {
			return dir, errors.Wrap(err, "pebbling graph error")
		}
		log.Println("calc layer:", i)
	}
	//calculate new dir name
	root, err := tree.CalculateMerkelTreeRoot(roots)
	if err != nil {
		return dir, errors.Wrap(err, "pebbling graph error")
	}
	hstr := hex.EncodeToString(root)
	if err = os.Rename(path.Join(dir), path.Join(rdir, hstr)); err != nil {
		return dir, errors.Wrap(err, "pebbling graph error")
	}
	dir = path.Join(rdir, hstr)
	if err = util.SaveProofFile(
		path.Join(dir, COMMIT_FILE),
		append(roots, root)); err != nil {
		return dir, errors.Wrap(err, "pebbling graph error")
	}
	return dir, nil
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

func Sort(nodes map[int64]struct{}) []int64 {
	var list []int64

	for k := range nodes {
		list = append(list, k)
	}
	sort.Sort(util.Int64s(list))
	return list
}
