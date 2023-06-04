package expanders

import "sync"

const (
	DEFAULT_ROUTINES = 32
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
