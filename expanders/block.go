package expanders

import (
	"cess_pos_demo/util"
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
)

const (
	DEFAULT_BLOCK_SIZE = 1024 * 4
	DEFAULT_BLOCK_NUM  = 256
	DEFAULT_DIR        = "./expanders"
	DEFAULT_BLOCK_NAME = "block"
)

type Block struct {
	rw       sync.RWMutex
	refCount int
	count    int
	nodes    []Node
}

func NewBlock() *Block {
	return &Block{
		nodes: make([]Node, 0),
	}
}

func (block *Block) Reference(num int) {
	block.rw.Lock()
	defer block.rw.Unlock()
	block.refCount++
}

func (block *Block) Deference() {
	block.rw.Lock()
	defer block.rw.Unlock()
	block.refCount--
}

func (block *Block) ReadRefCount() int {
	block.rw.RLock()
	defer block.rw.RUnlock()
	return block.refCount
}

func (block *Block) AddNode(node Node) {
	if len(block.nodes) <= 0 {
		block.nodes = make([]Node, DEFAULT_BLOCK_SIZE)
	}
	block.nodes[block.count] = node
	block.count++
}

func (block *Block) GetNode(index int) Node {
	if index < 0 || index > DEFAULT_BLOCK_SIZE {
		return Node{}
	}
	return block.nodes[index]
}

func (block *Block) Marshal() ([]byte, error) {

	jbytes, err := json.Marshal(block.nodes)
	return jbytes, errors.Wrap(err, "marshal block error")
}

func (block *Block) Unmarshal(data []byte) error {
	return errors.Wrap(json.Unmarshal(data, &block.nodes), "marshal block error")
}

func LoadBlock(path string) (*Block, error) {
	data, err := util.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "load block error")
	}
	block := NewBlock()
	if err := block.Unmarshal(data); err != nil {
		return nil, errors.Wrap(err, "load block error")
	}
	return block, nil
}

func SaveBlock(path string, block *Block) error {
	data, err := block.Marshal()
	if err != nil {
		return errors.Wrap(err, "save block error")
	}
	return errors.Wrap(util.SaveFile(path, data), "save block error")
}

type BlockQueue struct {
	rw    sync.RWMutex
	queue []*Block
}

var blockQueue *BlockQueue

func GetBlockQueue() *BlockQueue {
	if blockQueue == nil {
		blockQueue = &BlockQueue{
			queue: make([]*Block, DEFAULT_BLOCK_NUM),
		}
	}
	return blockQueue
}
