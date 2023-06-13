package test

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/expanders"
	"cess_pos_demo/pois"
	"cess_pos_demo/tree"
	"testing"
	"time"
)

func TestPois(t *testing.T) {
	//Initialize the execution environment
	graph := expanders.NewExpanders(7, 1024*1024, 64)
	tree.InitMhtPool(int(graph.N), expanders.HashSize)
	key := acc.RsaKeygen(2048)
	err := pois.InitProver(
		graph.K, graph.N, graph.D,
		[]byte("test miner id"),
		expanders.DEFAULT_IDLE_FILES_PATH,
		acc.DEFAULT_PATH, key,
	)
	if err != nil {
		t.Fatal("init prover error", err)
	}
	verifier := pois.NewVerifier(key, graph.K, graph.N, graph.D)
	prover := pois.GetProver()

	//run idle file generation server
	prover.RunIdleFileGenerationServer(pois.MaxCommitProofThread)

	//add file to generate
	ok := prover.GenerateFile(32)
	if !ok {
		t.Fatal("generate file error")
	}
	//wait 10 minutes for file generate
	time.Sleep(time.Minute * 10)
	ts := time.Now()

	//get commits
	commits, err := prover.GetCommits(16)
	if err != nil {
		t.Fatal("get commits error", err)
	}
	t.Log("get commits time", time.Since(ts))

	//register prover
	verifier.RegisterProverNode(prover.ID)

	//verifier receive commits
	ts = time.Now()
	if !verifier.ReceiveCommits(prover.ID, commits) {
		t.Fatal("receive commits error", err)
	}
	t.Log("verifier receive commits time", time.Since(ts))

	//generate commits challenges
	ts = time.Now()
	chals, err := verifier.CommitChallenges(prover.ID, 0, 16)
	if err != nil {
		t.Fatal("generate commit challenges error", err)
	}
	t.Log("generate commit challenges time", time.Since(ts))

	//prove commit
	ts = time.Now()
	commitProof, err := prover.ProveCommit(chals)
	if err != nil {
		t.Fatal("prove commit error", err)
	}
	t.Log("prove commit time", time.Since(ts))

	//verify commit proof
	ts = time.Now()
	//err =
	verifier.VerifyCommitProofs(prover.ID, chals, commitProof)
	if err != nil {
		t.Fatal("verify commit proof error", err)
	}
	t.Log("verify commit proof time", time.Since(ts))

	//acc proof

	ts = time.Now()
	idexs := make([]int64, len(chals))
	for i := 0; i < len(chals); i++ {
		idexs[i] = chals[i][0]
	}
	accproof, err := prover.ProveAcc(idexs)
	if err != nil {
		t.Fatal("prove acc proof error", err)
	}
	t.Log("prove acc proof time", time.Since(ts))

	//verify acc proof
	ts = time.Now()
	err = verifier.VerifyAcc(prover.ID, chals, accproof)
	if err != nil {
		t.Fatal("verify acc proof error", err)
	}
	t.Log("verify acc proof time", time.Since(ts))

	//add file to count
	ok = prover.UpdateCount(int64(len(chals)))
	if !ok {
		t.Fatal("update count error")
	}
	//generate space challenges
	ts = time.Now()
	spaceChals, err := verifier.SpaceChallenges(prover.ID, int64(len(chals)))
	if err != nil {
		t.Fatal("generate space chals error", err)
	}
	t.Log("generate space chals time", time.Since(ts))

	//prove space
	ts = time.Now()
	spaceProof, err := prover.ProveSpace(spaceChals)
	if err != nil {
		t.Fatal("prove space error", err)
	}
	t.Log("prove space time", time.Since(ts))

	//verify space proof
	ts = time.Now()
	err = verifier.VerifySpace(prover.ID, spaceChals, spaceProof)
	if err != nil {
		t.Fatal("verify space proof error", err)
	}
	t.Log("verify space proof time", time.Since(ts))

	//deletion proof
	ts = time.Now()
	chProof, Err := prover.ProveDeletion(1)
	var delProof *pois.DeletionProof
	select {
	case err = <-Err:
		t.Fatal("prove deletion proof error", err)
	case delProof = <-chProof:
		break
	}
	t.Log("prove deletion proof time", time.Since(ts))

	//verify deletion proof
	ts = time.Now()
	err = verifier.VerifyDeletion(prover.ID, delProof)
	if err != nil {
		t.Fatal("verify deletion proof error", err)
	}
	t.Log("verify deletion proof time", time.Since(ts))
}

// ts = time.Now()

// t.Log("verifier receive commits time", time.Since(ts))
