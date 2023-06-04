package test

import (
	"cess_pos_demo/expanders"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestExpandersGenerate(t *testing.T) {
	graph := expanders.ConstructStackedExpanders([]byte("test expanders"), 7, 256, 64, true)
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
	graph := expanders.ConstructStackedExpanders([]byte("test expanders"), 7, 1024*1024, 64, true)
	t.Log("construct stacked expanders time", time.Since(ts))
	ts = time.Now()
	wg := sync.WaitGroup{}
	wg.Add(64)
	for i := 0; i < 64; i++ {
		go func(count int) {
			defer wg.Done()
			path, err := graph.GenerateIdleFile([]byte("test miner id"), int64(count), expanders.DEFAULT_IDLE_FILES_PATH)
			if err != nil {
				t.Log("generate idle file", err)
			}
			t.Log("idle file path", path)
		}(i)
	}
	wg.Wait()
	t.Log("generate idle file time", time.Since(ts))
}
