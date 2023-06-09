package expanders

import "cess_pos_demo/util"

const (
	DEFAULT_ROUTINES = 8
)

var (
	BuffSize = 2
)

func ConstructStackedExpanders(k, n, d int64) *Expanders {
	return NewExpanders(k, n, d)
}

func CalcParents(expanders *Expanders, node *Node, MinerID []byte, Count int64) {
	if node == nil || expanders == nil || node.Index == 0 ||
		cap(node.Parents) != int(expanders.D+1) {
		return
	}
	lens := len(MinerID) + 8*17
	content := make([]byte, lens)
	util.CopyData(content, MinerID, GetBytes(Count))
	node.AddParent(node.Index - NodeType(expanders.N))

	plate := make([][]byte, 16)
	for i := 0; i < int(expanders.D/16); i += 16 {
		for j := 0; j < 16; j++ {
			plate[i+j] = GetBytes(int64(i + j))
		}
		util.CopyData(content[lens-8*16:], plate...)
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

func (expanders *Expanders) RunRelationalMapServer(MinerID []byte, Count int64) <-chan *Node {
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
