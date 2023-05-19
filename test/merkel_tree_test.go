package test

import (
	"cess_pos_demo/graph"
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"crypto/sha512"
	"testing"
	"time"
)

func TestMerkelTree(t *testing.T) {
	data := make([][]byte, 4*1024*1024)
	st := time.Now()
	for i := 0; i < 4*1024*1024; i++ {
		data[i] = graph.GetHash([]byte(util.RandString(64)))
	}
	t.Log("create data time:", time.Since(st))
	st = time.Now()
	root, err := tree.CalculateMerkelTreeRoot(data)
	if err != nil {
		t.Fatal("create merkel tree error", err)
	}
	t.Log("create tree time:", time.Since(st))
	st = time.Now()
	path, locs, err := tree.CalculateTreePath(data, 2)
	if err != nil {
		t.Fatal("get merkel path error", err)
	}
	t.Log("create path time:", time.Since(st))
	st = time.Now()
	h := sha512.New()
	h.Write(data[2])
	if tree.VerifyTreePath(path, locs, h.Sum(nil), root) {
		t.Log("verify path success.")
	}
	t.Log("verify path time:", time.Since(st))
}
