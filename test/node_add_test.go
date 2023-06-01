package test

import (
	"cess_pos_demo/expanders"
	"crypto/rand"
	"math/big"
	"testing"
)

func TestAddNode(t *testing.T) {
	node := expanders.NewNode(1)
	for i := 0; i < 100; i++ {
		p, _ := rand.Int(rand.Reader, big.NewInt(1024))
		if node.AddParent(expanders.NodeType(p.Int64() + 1)) {
			t.Log("add parent success", p.Int64()+1)
		}
	}
	t.Log(node.Parents)
}
