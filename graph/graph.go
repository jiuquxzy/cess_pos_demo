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

var (
	LeveldbThread = 16
)

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

func NewStackedExpanders(path string, n, k, d int64) (*StackedExpanders, error) {
	sg := &StackedExpanders{
		N: n,
		K: k,
		D: d,
	}
	g, err := NewGraph(path, n*(k+1))
	if err != nil {
		return nil, err
	}
	sg.Graph = g
	return sg, nil
}

func (g *StackedExpanders) MarshalGraph() ([]byte, error) {
	return json.Marshal(g)
}

func UnmarshalGraph(data []byte) (*StackedExpanders, error) {
	g := &StackedExpanders{
		Graph: new(Graph),
	}
	err := json.Unmarshal(data, g)
	if err != nil {
		return nil, err
	}
	g.db, err = leveldb.OpenFile(g.Path, nil)
	g.batch = new(leveldb.Batch)
	if err != nil {
		return nil, err
	}
	return g, nil
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
	node.UnmarshalBinary(data)
	if node.Index != idx {
		return nil, fmt.Errorf("expected node with idx=%d; got idx=%d", idx, node.Index)
	}
	return node, nil
}

func (g *Graph) PutNode(node *Node) error {
	data := node.MarshalBinary()
	err := g.db.Put(util.Int64Bytes(node.Index), data, nil)
	if err != nil {
		log.Println("put node to db error", err)
	}
	return err
}

func (g *Graph) putBatch(node *Node) {
	data := node.MarshalBinary()
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

// Adapted from "Proof of Space from Stacked Expanders", 2016 (Ren, Devadas)
func ConstructStackedExpanders(path string, n, k, d int64, localize bool) (*StackedExpanders, error) {
	size := n * (k + 1)
	graph, err := NewGraph(path, size)
	if err != nil {
		return nil, errors.Wrap(err, "construct stacked expanders error")
	}
	//add first layer nodes into leveldb
	wg := sync.WaitGroup{}
	wg.Add(LeveldbThread)
	for i := 0; i < LeveldbThread; i++ {
		go func(i int64) {
			defer wg.Done()
			a := i * n / int64(LeveldbThread)
			b := (i + 1) * n / int64(LeveldbThread)
			for j := a; j < b; j++ {
				graph.PutNode(NewNode(j))
			}
		}(int64(i))
	}
	wg.Wait()
	//add no-source nodes with their parents map into leveldb
	wg = sync.WaitGroup{}
	for m := n; m < size; m += n {
		wg.Add(1)
		go func(m int64) {
			defer wg.Done()
			PinskerExpander(graph, m, n, d, localize)
		}(m)
	}
	wg.Wait()
	return &StackedExpanders{
		Graph: graph,
		K:     k, N: n, D: d,
	}, nil
}

func PinskerExpander(g *Graph, m, n, d int64, localize bool) {
	wg := sync.WaitGroup{}
	wg.Add(LeveldbThread)
	for i := 0; i < LeveldbThread; i++ {
		go func(i int64) {
			defer wg.Done()
			a := m + i*n/int64(LeveldbThread)
			b := m + (i+1)*n/int64(LeveldbThread)
			for sink := a; sink < b; sink++ {
				node := NewNode(sink)
				for count := int64(0); count < d; {
					src := util.Rand(n) + m - n
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
				if err := g.PutNode(node); err != nil {
					return
				}
			}
		}(int64(i))
	}
	wg.Wait()
}
