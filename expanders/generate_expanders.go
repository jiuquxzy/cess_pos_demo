package expanders

import (
	"sync"
)

const (
	DEFAULT_ROUTINES = 8
)

var (
	BuffSize = 2
)

func ConstructStackedExpanders(expandersID []byte, k, n, d int64, localize bool) *Expanders {
	expanders := NewExpanders(k, n, d, expandersID)
	//add first layer nodes into leveldb
	expanders.Nodes = make([]*Node, n)
	GenerateRelationalMap(expanders, localize)
	return expanders
}

func GenerateRelationalMap(expanders *Expanders, localize bool) {
	rmap := expanders.GetRelationalMap()
	wg := sync.WaitGroup{}
	wg.Add(DEFAULT_ROUTINES)
	for i := 0; i < DEFAULT_ROUTINES; i++ {
		go func(i int64) {
			defer wg.Done()
			left := i * expanders.N / DEFAULT_ROUTINES
			right := (i + 1) * expanders.N / DEFAULT_ROUTINES
			random := RandFunc(NodeType(expanders.N))
			for j := left; j < right; j++ {
				(*rmap)[j] = NewNode(NodeType(j + expanders.N))
				(*rmap)[j].Parents = make([]NodeType, 0, expanders.D+1)
				for count := int64(0); count < expanders.D; {
					src := random()
					if localize && src < NodeType(j) {
						src += NodeType(expanders.N)
					}
					if (*rmap)[j].AddParent(src) {
						count++
					}
				}
				if localize {
					(*rmap)[j].AddParent(NodeType(j))
				}
			}
		}(int64(i))
	}
	wg.Wait()
}

func CalcParents(expanders *Expanders, node *Node, MinerID []byte, Count int64) {
	if node == nil || expanders == nil || node.Index == 0 ||
		cap(node.Parents) != int(expanders.D+1) {
		return
	}
	lens := len(MinerID) + 8*17
	content := make([]byte, lens)
	copyData(content, MinerID, GetBytes(Count))
	node.AddParent(node.Index - NodeType(expanders.N))

	plate := make([][]byte, 16)
	for i := 0; i < int(expanders.D/16); i += 16 {
		for j := 0; j < 16; j++ {
			plate[i+j] = GetBytes(int64(i + j))
		}
		copyData(content[lens-8*16:], plate...)
		hash := GetHash(content)
		s, p := 0, NodeType(0)
		for j := 0; j < 16; {
			if s < 4 {
				p = BytesToNodeValue(hash[j*4+s:(j+1)*4+s], expanders.N)
			} else {
				s = 0
				for {
					p = (p + 1) % NodeType(expanders.N)
					if _, ok := node.ParentInList(p); !ok {
						break
					}
				}
			}
			if p < node.Index-NodeType(expanders.N) {
				p += NodeType(expanders.N)
			}
			if node.AddParent(p) {
				j++
				s = 0
				continue
			}
			s++
		}
	}
}

func RunRelationalMapServer(expanders *Expanders, MinerID []byte, Count int64) <-chan *Node {
	buf := make([]*Node, BuffSize)
	for i := 0; i < BuffSize; i++ {
		buf[i] = NewNode(0)
		buf[i].Parents = make([]NodeType, 0, expanders.N)
	}
	out := make(chan *Node, BuffSize/2)
	go func() {
		for l := int64(0); l <= expanders.K; l++ {
			for index := int64(0); index < expanders.N; index++ {
				node := buf[index%int64(BuffSize)]
				node.Index = NodeType(index + expanders.N)
				node.Parents = node.Parents[:0]
				CalcParents(expanders, node, MinerID, Count)
				out <- node
			}
		}
	}()
	return out
}
