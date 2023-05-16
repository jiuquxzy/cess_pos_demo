package graph

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"cess_pos_demo/util"

	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
)

/*
This package is implemented with reference to (https://github.com/zbo14/pos/graph)
*/

type Graph struct {
	Size  int64 `json:"size"`
	db    *leveldb.DB
	batch *leveldb.Batch
	Path  string `json:"path"`
}

type StackedExpanders struct {
	*Graph
	N, K, D int64
}

func NewGraph(path string, size int64) (*Graph, error) {
	var err error
	g := new(Graph)
	g.Path = path
	g.batch = new(leveldb.Batch)
	g.db, err = leveldb.OpenFile(path, nil)
	g.Size = size
	if err != nil {
		log.Printf("open leveldb from %s error %v\n", path, err)
	}
	return g, err
}

func MarshalGraphToJson(graph *Graph) (string, error) {
	bytes, err := json.Marshal(graph)
	if err != nil {
		log.Println("marshal graph to json string error", err)
		return "", err
	}
	return string(bytes), nil
}

func UnmarshalGraph(s string) (*Graph, error) {
	graph := new(Graph)
	err := json.Unmarshal([]byte(s), graph)
	if err != nil {
		log.Println("unmarshal graph error", err)
		return nil, err
	}
	graph.batch = new(leveldb.Batch)
	graph.db, err = leveldb.OpenFile(graph.Path, nil)
	if err != nil {
		log.Printf("open leveldb from %s error %v\n", graph.Path, err)
	}
	return graph, err
}

func (g *Graph) GetNode(idx int64) (*Node, error) {
	if idx < 0 || idx >= g.Size {
		log.Println("index out of range")
		return nil, errors.New("index out of range")
	}
	data, err := g.db.Get(util.Int64Bytes(idx), nil)
	if err != nil {
		log.Println("get node from db error", err)
		return nil, err
	}
	node := new(Node)
	if err = json.Unmarshal(data, node); err != nil {
		log.Println("unmarshal node error", err)
		return nil, err
	}
	if node.Index != idx {
		return nil, fmt.Errorf("expected node with idx=%d; got idx=%d", idx, node.Index)
	}
	return node, nil
}

func (g *Graph) PutNode(node *Node) error {
	data, err := json.Marshal(node)
	if err != nil {
		log.Println("marshal node error", err)
		return err
	}
	err = g.db.Put(util.Int64Bytes(node.Index), data, nil)
	if err != nil {
		log.Println("put node to db error", err)
	}
	return err
}

func (g *Graph) putBatch(node *Node) {
	data, _ := json.Marshal(node)
	g.batch.Put(util.Int64Bytes(node.Index), data)
}

func (g *Graph) writeBatch() error {
	err := g.db.Write(g.batch, nil)
	if err != nil {
		log.Println("write batch error", err)
		return err
	}
	g.batch = new(leveldb.Batch)
	return nil
}

func (g *Graph) GetParents(idx int64) (map[int64]struct{}, error) {
	node, err := g.GetNode(idx)
	if err != nil {
		return nil, err
	}
	return node.Parents, nil
}

//Adapted from "Proof of Space from Stacked Expanders", 2016 (Ren, Devadas)

func ConstructStackedExpanders(path string, n, k, d int64, localize bool) (*StackedExpanders, error) {
	size := n * (k + 1)
	graph, err := NewGraph(path, size)
	if err != nil {
		return nil, errors.Wrap(err, "construct stacked expanders error")
	}
	for idx := int64(0); idx < size; idx++ {
		graph.putBatch(NewNode(idx))
	}
	graph.writeBatch()
	wg := sync.WaitGroup{}
	for m := int64(0); m < size-n; m += n {
		wg.Add(1)
		go func(m int64) {
			defer wg.Done()
			if err := PinskerExpander(graph, m, n, d, localize); err != nil {
				log.Println("construct stacked expanders error", err)
			}
		}(m)
	}
	wg.Wait()
	return &StackedExpanders{
		Graph: graph,
		K:     k, N: n, D: d,
	}, nil
}

func PinskerExpander(g *Graph, m, n, d int64, localize bool) error {
	batch := new(leveldb.Batch)
	for sink := m + n; sink < m+2*n; sink++ {
		node, err := g.GetNode(sink)
		if err != nil {
			return errors.Wrap(err, "construct pinsker expander error")
		}
		for count := int64(0); count < d; {
			src := util.Rand(n) + m
			if localize && src+n < node.Index {
				src = src + n
			}
			if node.AddParent(src) {
				count++
			}
		}
		if localize {
			node.AddParent(node.Index - n)
		}
		data, err := json.Marshal(node)
		if err != nil {
			return errors.Wrap(err, "construct pinsker expander error")
		}
		batch.Put(util.Int64Bytes(node.Index), data)

	}
	return errors.Wrap(g.db.Write(batch, nil), "construct pinsker expander error")
}
