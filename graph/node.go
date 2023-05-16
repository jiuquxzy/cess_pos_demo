package graph

type Node struct {
	Index   int64              `json:"index"`
	Parents map[int64]struct{} `json:"parents"`
}

func NewNode(idx int64) *Node {
	return &Node{
		Index:   idx,
		Parents: make(map[int64]struct{}),
	}
}

func (node *Node) AddParent(parent int64) bool {
	if node.Index == parent {
		return false
	}
	if _, ok := node.Parents[parent]; ok {
		return false
	}
	node.Parents[parent] = struct{}{}
	return true
}

func (node Node) NoParents() bool {
	return len(node.Parents) == 0
}
