package test

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/graph"
	"cess_pos_demo/protocol"
	"testing"
	"time"
)

func TestProtocol(t *testing.T) {
	key := acc.RsaKeygen(2048)
	id := []byte("this is a account id")
	gpath := "graph/stacked_expanders/g003"
	prover := protocol.NewProver(key, id)
	//get graph
	err := prover.NewGraph(gpath, 1024*1024, 7, 64, true)
	if err != nil {
		t.Fatal("create new graph error", err)
	}
	//commit params
	params := prover.GetParams()
	verifyer := protocol.NewVerifier(key, params.K, params.N, params.D)
	//create file
	st := time.Now()
	fpath, err := prover.CreateIdleFile(graph.DEFAULT_PATH)
	if err != nil {
		t.Fatal("create new idle file error", err)
	}
	t.Log("file path ", fpath)
	t.Log("create file time:", time.Since(st))
	//commit proof
	st = time.Now()
	proofs, err := prover.ReadCommitProof(int(prover.Count))
	if err != nil {
		t.Fatal("read commit proof error", err)
	}
	t.Log("read proof time:", time.Since(st))
	st = time.Now()
	if !verifyer.ReceiveCommits(prover.ID, proofs) {
		t.Fatal("receive commit proof error", len(proofs))
	}
	t.Log("receive and verify proof time:", time.Since(st))
	st = time.Now()
	//commit challenge
	cChals, err := verifyer.CommitChallenges()
	if err != nil {
		t.Fatal("generate commit challenge error", err)
	}
	t.Log("create commit challenges time:", time.Since(st))
	st = time.Now()
	//prove commit
	proof, err := prover.ProveCommit(fpath, cChals)
	if err != nil {
		t.Fatal("generate commit proof error", err)
	}
	t.Log("prove commit proof time:", time.Since(st))
	st = time.Now()
	ok, err := verifyer.VerifyCommit(prover.ID, cChals, proof)
	if err != nil {
		t.Fatal("verify commit proof error", err)
	}
	t.Log("verify commit result:", ok)
	t.Log("verify commit proof time:", time.Since(st))
	st = time.Now()
	//space challenge
	sChals, err := verifyer.SpaceChallenges(1, 1)
	if err != nil {
		t.Fatal("generate space challenge error", err)
	}
	t.Log("create space challenges time:", time.Since(st))
	st = time.Now()
	//prove space
	sproof, err := prover.ProveSpace(sChals)
	if err != nil {
		t.Fatal("prover space proof error", err)
	}
	t.Log("prove space proof time:", time.Since(st))
	st = time.Now()
	ok, err = verifyer.VerifySpace(prover.ID, sChals, sproof)
	if err != nil {
		t.Fatal("verify space proof error", err)
	}
	t.Log("verify space result:", ok)
	t.Log("verify space proof time:", time.Since(st))
	st = time.Now()
	//prove delete
	dproof, err := prover.ProveDeletion()
	if err != nil {
		t.Fatal("prove deletion error", err)
	}
	t.Log("prove deletion proof time:", time.Since(st))
	st = time.Now()
	ok, err = verifyer.VerifyDeletion(prover.ID, dproof)
	if err != nil {
		t.Fatal("verify deletion proof error", err)
	}
	t.Log("verify deletion result:", ok)
	t.Log("verify deletion proof time:", time.Since(st))
}
