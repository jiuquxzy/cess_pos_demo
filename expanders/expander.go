package expanders

type NodeType int32

type Expanders struct {
	Size int64  `json:"size"`
	Path string `json:"path"`
}

type Node struct {
	Index   NodeType   `json:"index"`
	Parents []NodeType `json:"parents"`
}

func NewNode(idx NodeType) *Node {
	return &Node{Index: idx}
}

func (node *Node) AddParent(parent NodeType) bool {
	if node.Index == parent {
		return false
	}
	i, ok := node.ParentInList(parent)
	if ok {
		return false
	}
	if node.Parents == nil {
		node.Parents = append(node.Parents, parent)
		return true
	}
	after := node.Parents[i:]
	node.Parents = append([]NodeType{}, node.Parents[:i]...)
	node.Parents = append(node.Parents, parent)
	node.Parents = append(node.Parents, after...)
	return true
}

func (node *Node) NoParents() bool {
	return len(node.Parents) <= 0
}

func (node *Node) ParentInList(parent NodeType) (int, bool) {
	if node.NoParents() {
		return 0, false
	}
	lens := len(node.Parents)
	l, r := 0, lens-1
	for l <= r {
		mid := (l + r) / 2
		if node.Parents[mid] == parent {
			return 0, true
		}
		if node.Parents[mid] > parent {
			r = mid - 1
		} else {
			l = mid + 1
		}
	}
	i := (l + r) / 2
	if node.Parents[i] < parent {
		i++
	}
	return i, false
}
