package test

import (
	"cess_pos_demo/expanders"
	"cess_pos_demo/tree"
	"encoding/json"
	_ "net/http/pprof"
	"sync"
	"testing"
	"time"
)

func TestExpandersGenerate(t *testing.T) {
	graph := expanders.ConstructStackedExpanders(7, 256, 64)
	err := graph.MarshalAndSave(expanders.DEFAULT_EXPANDERS_PATH)
	if err != nil {
		t.Fatal("test marshal expanders error", err)
	}
	graph, err = expanders.ReadAndUnmarshalExpanders(expanders.DEFAULT_EXPANDERS_PATH)
	if err != nil {
		t.Fatal("test unmarshal expanders error", err)
	}
	bytes, err := json.Marshal(graph)
	if err != nil {
		t.Fatal("marshal new expanders error", err)
	}
	t.Log("expanders", string(bytes))
}

func TestIdleFileGeneration(t *testing.T) {
	ts := time.Now()
	expanders.InitLabelsPool(1024*1024, int64(expanders.HashSize))
	tree.InitMhtPool(1024*1024, expanders.HashSize)
	graph := expanders.ConstructStackedExpanders(7, 1024*1024, 64)
	t.Log("construct stacked expanders time", time.Since(ts))
	tree.InitMhtPool(1024*1024, expanders.HashSize)
	ts = time.Now()
	wg := sync.WaitGroup{}
	wg.Add(16)
	for i := 0; i < 16; i++ {
		go func(count int) {
			defer wg.Done()
			err := graph.GenerateIdleFile([]byte("test miner id"), int64(i+1), expanders.DEFAULT_IDLE_FILES_PATH)
			if err != nil {
				t.Log("generate idle file", err)
			}
		}(i)
	}
	wg.Wait()
	t.Log("generate idle file time", time.Since(ts))
}

func TestRealationMap(t *testing.T) {
	graph := expanders.ConstructStackedExpanders(3, 1024, 64)
	ch := graph.RunRelationalMapServer([]byte("test ud"), 1)
	count := 0
	for node := range ch {
		if count > 10 {
			break
		}
		t.Log("node index", node.Index)
		count++
	}
}
