package test

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/graph"
	"cess_pos_demo/util"
	"sync"
	"testing"
	"time"
)

func TestACC(t *testing.T) {
	data := make([][]byte, 1024)
	for i := 0; i < len(data); i++ {
		data[i] = graph.GetHash([]byte(util.RandString(64)))
	}
	wg := sync.WaitGroup{}
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			key := acc.RsaKeygen(2048)
			st := time.Now()
			acc.GenerateAcc(key, data)
			t.Log("create acc time:", time.Since(st))
			st = time.Now()
			wit := acc.GenerateWitness(key.G, key.N, data)
			byteCount := 0
			for i := 0; i < len(wit); i++ {
				byteCount += len(wit[i])
			}
			t.Log("calc witness time:", time.Since(st), byteCount)
		}()
	}
	wg.Wait()
}
