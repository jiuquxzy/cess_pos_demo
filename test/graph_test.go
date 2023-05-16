package test

import (
	"cess_pos_demo/graph"
	"testing"
	"time"
)

func TestGraph(t *testing.T) {
	gpath := "graph/stacked_expanders/g001"
	id := []byte("this is a account id")
	st := time.Now()
	stackedGraph, err := graph.ConstructStackedExpanders(gpath, 1024*1024*4, 31, 128, true)
	if err != nil {
		t.Fatal("construct stacked expanders error", err)
	}
	t.Log("construct stacked expanders time:", time.Since(st))
	st = time.Now()
	path, err := graph.PebblingGraph(stackedGraph, id, 1, graph.DEFAULT_PATH)
	if err != nil {
		t.Fatal("pebbling stacked expanders error", err)
	}
	t.Log("construct stacked expanders time:", time.Since(st))
	t.Log("proof file path", path)
}
