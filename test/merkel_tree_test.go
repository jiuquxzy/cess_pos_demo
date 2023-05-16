package test

import (
	"cess_pos_demo/tree"
	"crypto/sha512"
	"testing"
)

func TestMerkelTree(t *testing.T) {
	data := [][]byte{
		[]byte("aaa"),
		[]byte("bbb"),
		[]byte("ccc"),
		[]byte("ddd"),
	}
	root, err := tree.CalculateMerkelTreeRoot(data)
	if err != nil {
		t.Fatal("create merkel tree error", err)
	}
	path, locs, err := tree.CalculateTreePath(data, 2)
	if err != nil {
		t.Fatal("get merkel path error", err)
	}
	h := sha512.New()
	h.Write(data[2])
	if tree.VerifyTreePath(path, locs, h.Sum(nil), root) {
		t.Log("verify path success.")
	}
}
