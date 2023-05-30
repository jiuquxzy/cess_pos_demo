package test

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/graph"
	"cess_pos_demo/protocol"
	"log"
	"sync"
	"testing"
	"time"
)

func TestGenerateIdleFile(t *testing.T) {
	key := acc.RsaKeygen(2048)
	id := []byte("this is a account id")
	gpath := "graph/stacked_expanders/g003"
	prover := protocol.NewProver(key, id)
	//get graph
	err := prover.SetGraph(gpath, 1024*1024, 7, 64)
	if err != nil {
		t.Fatal("create new graph error", err)
	}
	st := time.Now()
	wg := sync.WaitGroup{}
	wg.Add(16)
	for i := 0; i < 16; i++ {
		go func(i int64) {
			defer wg.Done()
			fpath, err := graph.PebblingGraph(prover.GetGraph(), id, i, graph.DEFAULT_PATH)
			if err != nil {
				log.Println("create file error", err)
				return
			}
			log.Println("file path ", fpath)
		}(int64(i))
	}
	wg.Wait()
	t.Log("create file time:", time.Since(st))
}
