package tree

import (
	"bytes"
	"crypto/sha512"
	"errors"

	"github.com/CESSProject/go-merkletree"
)

type Data []byte

func (d Data) CalculateHash() ([]byte, error) {
	h := sha512.New()
	if _, err := h.Write(d); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func (d Data) Equals(other merkletree.Content) (bool, error) {
	o, ok := other.(Data)
	if !ok {
		return false, errors.New("type mismatch")
	}
	if len(d) != len(o) {
		return false, nil
	}
	for i := 0; i < len(d); i++ {
		if d[i] != o[i] {
			return false, nil
		}
	}
	return true, nil
}

func CalculateMerkelTreeRoot(data [][]byte) ([]byte, error) {
	t, err := CalculateMerkelTree(data)
	if err != nil {
		return nil, err
	}
	return t.MerkleRoot(), nil
}

func CalculateMerkelTreeRoot2(data []byte, lens int) ([]byte, error) {
	datas := make([][]byte, len(data)/lens)
	for i := 0; i < len(datas); i++ {
		datas[i] = data[i*lens : (i+1)*lens]
	}
	t, err := CalculateMerkelTree(datas)
	if err != nil {
		return nil, err
	}
	return t.MerkleRoot(), nil
}

func CalculateTreePath(data [][]byte, index int) ([][]byte, []int64, error) {
	if index < 0 || index > len(data) {
		return nil, nil, errors.New("index out of range")
	}
	t, err := CalculateMerkelTree(data)
	if err != nil {
		return nil, nil, err
	}
	return t.GetMerklePath(Data(data[index]))
}

func CalculateTreePathWitTree(tree *merkletree.MerkleTree, leaf []byte) ([][]byte, []int64, error) {
	return tree.GetMerklePath(Data(leaf))
}

func VerifyTreePath(path [][]byte, locs []int64, node, root []byte) bool {
	h := sha512.New()
	for k := 0; k < len(path); k++ {
		if locs[k] == 1 {
			node = append(node, path[k]...)
		} else {
			node = append(path[k], node...)
		}
		h.Reset()
		h.Write(node)
		node = h.Sum(nil)
	}
	return bytes.Equal(node, root)
}

func CalculateMerkelTree(data [][]byte) (*merkletree.MerkleTree, error) {
	list := make([]merkletree.Content, len(data))
	for i := 0; i < len(data); i++ {
		list[i] = Data(data[i])
	}
	return merkletree.NewTreeWithHashStrategy(list, sha512.New)
}
