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
	data := make([][]byte, 1024*1024)
	st := time.Now()
	for i := 0; i < 1024*1024; i++ {
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

func TestLightMerkelTree(t *testing.T) {
	size := 64
	data := make([]byte, 64*size)

	st := time.Now()
	for i := 0; i < 64; i++ {
		copy(data[i*size:(i+1)*size], graph.GetHash([]byte(util.RandString(64))))
	}
	t.Log("create data time:", time.Since(st))
	st = time.Now()
	ltree := tree.CalcLightMmtWitBytes(data, size)
	t.Log("create tree time:", time.Since(st))
	t.Log("len", len(ltree)/size)
	proof, err := ltree.GetPathProof(data, 5, size)
	if err != nil {
		t.Fatal("get merkel path error", err)
	}
	t.Log("proof", proof)
	if tree.VerifyPathProof(ltree.GetRoot(size), data[5*64:6*64], proof) {
		t.Log("verify path proof success")
	} else {
		t.Log("verify path proof failure")
	}
}
