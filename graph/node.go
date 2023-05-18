package graph

import (
	"cess_pos_demo/util"
	"encoding/binary"
)

type Node struct {
	Index   int64              `json:"index"`
	Parents map[int64]struct{} `json:"parents"`
}

func NewNode(idx int64) *Node {
	return &Node{
		Index:   idx,
		Parents: make(map[int64]struct{}),
	}
}

func (node *Node) AddParent(parent int64) bool {
	if node.Index == parent {
		return false
	}
	if _, ok := node.Parents[parent]; ok {
		return false
	}
	node.Parents[parent] = struct{}{}
	return true
}

func (node Node) NoParents() bool {
	return len(node.Parents) == 0
}

// MarshalBinary marshal node to bytes（just be used to save in leveldb）
func (node *Node) MarshalBinary() []byte {
	data := make([]byte, 8*(len(node.Parents)+1))
	copy(data[:8], util.Int64Bytes(node.Index))
	n := 8
	for p := range node.Parents {
		copy(data[n:n+8], util.Int64Bytes(p))
		n += 8
	}
	return data
}

// UnmarshalBinary unmarshal node with bytes
func (node *Node) UnmarshalBinary(data []byte) {
	// Note: data should not be modified
	node.Index, _ = binary.Varint(data[:8])
	node.Parents = make(map[int64]struct{})
	if len(data) <= 8 {
		return
	}
	for i := 8; i < len(data); i += 8 {
		p, _ := binary.Varint(data[i : i+8])
		node.Parents[p] = struct{}{}
	}
}
