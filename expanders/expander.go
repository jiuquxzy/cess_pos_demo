package expanders

import (
	"bytes"
	"cess_pos_demo/util"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	DEFAULT_EXPANDERS_PATH = "./expanders/default_expanders"
)

type NodeType int32

type Expanders struct {
	K, N, D int64
	ID      []byte
	Size    int64   `json:"size"`
	Nodes   []*Node `json:"nodes"`
}

type Node struct {
	Index   NodeType   `json:"index"`
	Parents []NodeType `json:"parents"`
}

func (expanders *Expanders) MarshalAndSave(path string) error {
	data, err := json.Marshal(expanders)
	if err != nil {
		return errors.Wrap(err, "marshal and save expanders error")
	}
	err = util.SaveFile(path, data)
	return errors.Wrap(err, "marshal and save expanders error")
}

func ReadAndUnmarshalExpanders(path string) (*Expanders, error) {
	data, err := util.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read and unmarshal expanders error")
	}
	expander := new(Expanders)
	err = json.Unmarshal(data, expander)
	return expander, errors.Wrap(err, "read and unmarshal expanders error")
}

func NewExpanders(k, n, d int64, ID []byte) *Expanders {
	return &Expanders{
		ID:   ID,
		Size: (k + 1) * n,
		K:    k, N: n, D: d,
	}
}

func (expanders *Expanders) GetRelationalMap() *[]*Node {
	return &expanders.Nodes
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

func GetBytes(v NodeType) []byte {
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, v)
	return bytesBuffer.Bytes()
}

func RandFunc(max NodeType) func() NodeType {
	len := unsafe.Sizeof(max)
	buf := make([]byte, len)
	return func() NodeType {
		rand.Read(buf)
		value, _ := binary.Varint(buf)
		if value < 0 {
			value *= -1
		}
		return NodeType(value) % max
	}
}
