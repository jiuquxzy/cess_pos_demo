package test

import (
	"cess_pos_demo/graph"
	"testing"
)

func TestMarshalGraph(t *testing.T) {
	gpath := "graph/stacked_expanders/g006"
	g, err := graph.ConstructStackedExpanders(gpath, 1024, 7, 8, true)
	if err != nil {
		t.Fatal("construct stacked expanders error", err)
	}
	bytes, err := g.MarshalGraph()
	if err != nil {
		t.Fatal("marshal stacked expanders error", err)
	}
	t.Log("graph:", string(bytes))
	g, err = graph.UnmarshalGraph(bytes)
	if err != nil {
		t.Fatal("unmarshal stacked expanders error", err)
	}
	t.Log("graph:", g)
}
