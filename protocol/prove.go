package protocol

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/graph"
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"fmt"
	"log"
	"path"
	"sync"

	"github.com/CESSProject/go-merkletree"
	"github.com/pkg/errors"
)

var (
	ProofBufSize = 1024
)

type Prover struct {
	graph      *graph.StackedExpanders
	Count      int64
	ID         []byte
	FilePath   []string
	AccManager *acc.AccManager
	TokenChan  chan struct{}
}

type MerkelTreeProof struct {
	Index   int64
	Label   []byte
	Paths   [][]byte
	Locs    []int64
	Parents []MerkelTreeProof
}

type MerkelProofItem struct {
	Index int64
	Label []byte
	Paths [][]byte
	Locs  []int64
}

type CommitProofItem struct {
	Node    MerkelProofItem
	Parents []MerkelProofItem
}

type CommitProof struct {
	Proofs []CommitProofItem
	ACC    []byte
}

type ProofParams struct {
	ID      []byte
	K, N, D int64
}

type SpaceProof struct {
	Proofs []MerkelProofItem
	Roots  [][]byte
	Index  []int64
	Wits   [][]byte
}

type DeletionProof struct {
	Acc  []byte
	Root []byte
}

func NewProver(key acc.RsaKey, id []byte) *Prover {
	p := &Prover{
		ID:         id,
		AccManager: acc.NewAccManager(key),
		TokenChan:  make(chan struct{}, ProofBufSize),
	}
	for i := 0; i < ProofBufSize; i++ {
		p.TokenChan <- struct{}{}
	}
	return p
}

func (p *Prover) NewGraph(path string, n int64, k int64, d int64, localize bool) error {
	var err error
	p.graph, err = graph.ConstructStackedExpanders(path, n, k, d, localize)
	return err
}

func (p *Prover) SetGraph(path string, n int64, k int64, d int64) error {
	var err error
	p.graph, err = graph.NewStackedExpanders(path, n, k, d)
	return err
}

func (p *Prover) GetParams() *ProofParams {
	return &ProofParams{
		ID: p.ID,
		K:  p.graph.K,
		N:  p.graph.N,
		D:  p.graph.D,
	}
}

func (p *Prover) CreateIdleFile(rdir string) (string, error) {
	path, err := graph.PebblingGraph(p.graph, p.ID, p.Count+1, rdir)
	if err != nil {
		return path, errors.Wrap(err, "create file error")
	}
	p.FilePath = append(p.FilePath, path)
	p.Count++
	return path, nil
}

func (p Prover) GetGraph() *graph.StackedExpanders {
	return p.graph
}

func (p *Prover) AddIdleFile(path string) {
	p.Count++
	p.FilePath = append(p.FilePath, path)
}

func (p *Prover) ReadCommitProof(fidx int) ([][]byte, error) {
	if fidx <= 0 || fidx > len(p.FilePath) {
		return nil, errors.New("create commit proof error index out of range")
	}
	dir := p.FilePath[fidx-1]
	fpath := path.Join(dir, graph.COMMIT_FILE)
	data, err := util.ReadProofFile(fpath, int(p.graph.K+2), graph.HashSize)
	if err != nil {
		return nil, errors.Wrap(err, "create commit proof error")
	}
	return data, nil
}

func (p *Prover) ProveCommit(fdir string, challenges map[int64]struct{}) (*CommitProof, error) {
	proofs := make([]CommitProofItem, len(challenges))
	i := 0
	wg := sync.WaitGroup{}
	for k := range challenges {
		<-p.TokenChan
		wg.Add(1)
		go func(k int64, i int) {
			defer func() { p.TokenChan <- struct{}{} }()
			defer wg.Done()
			proof, err := p.GenerateCommitProof(fdir, k)
			if err != nil {
				log.Println(errors.Wrap(err, "prove commit error"))
				return
			}
			proofs[i] = *proof
		}(k, i)
		i++
	}
	wg.Wait()
	data, err := util.ReadProofFile(
		path.Join(fdir, graph.COMMIT_FILE),
		int(p.graph.K+2), graph.HashSize,
	)
	if err != nil {
		return nil, errors.Wrap(err, "prove commit error")
	}
	label := append(p.ID, util.Int64Bytes(p.Count)...)
	label = append(label, data[p.graph.K]...)
	hash := graph.GetHash(label)
	p.AccManager.AddMember(hash)
	return &CommitProof{
		Proofs: proofs,
		ACC:    p.AccManager.Acc,
	}, nil
}

func (p *Prover) GenerateCommitProof(fdir string, c int64) (*CommitProofItem, error) {
	layer := c / p.graph.N
	if layer < 0 || layer > p.graph.K {
		return nil, errors.New("generate commit proof error: bad node index")
	}
	fpath := path.Join(fdir, fmt.Sprintf("%s-%d", graph.LAYER_NAME, layer))
	data, err := util.ReadProofFile(fpath, int(p.graph.N), graph.HashSize)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	var nodeTree, parentTree *merkletree.MerkleTree
	index := c % p.graph.N
	nodeTree, err = tree.CalculateMerkelTree(data)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	treePath, locs, err := tree.CalculateTreePathWitTree(nodeTree, data[int(index)])
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	proof := &CommitProofItem{
		Node: MerkelProofItem{
			Index: c,
			Label: data[index],
			Paths: treePath,
			Locs:  locs,
		},
	}
	if layer == 0 {
		return proof, nil
	}
	parents, err := p.graph.GetParents(c)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	fpath = path.Join(fdir, fmt.Sprintf("%s-%d", graph.LAYER_NAME, layer-1))
	pdata, err := util.ReadProofFile(fpath, int(p.graph.N), graph.HashSize)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	parentTree, err = tree.CalculateMerkelTree(pdata)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	list := graph.Sort(parents)
	parentProofs := make([]MerkelProofItem, len(list))
	wg := sync.WaitGroup{}
	wg.Add(len(list))
	mu := sync.Mutex{}
	for i := 0; i < len(list); i++ {
		go func(i int) {
			defer wg.Done()
			var label []byte
			index := list[i] % p.graph.N
			if list[i] >= layer*p.graph.N {
				label = data[index]
				treePath, locs, err = tree.CalculateTreePathWitTree(nodeTree, label)
			} else {
				label = pdata[index]
				treePath, locs, err = tree.CalculateTreePathWitTree(parentTree, label)
			}
			if err != nil {
				mu.Lock()
				err = errors.Wrap(err, "generate commit proof error")
				mu.Unlock()
				return
			}
			parentProofs[i] = MerkelProofItem{
				Index: list[i],
				Label: label,
				Paths: treePath,
				Locs:  locs,
			}
		}(i)
	}
	wg.Wait()
	if err != nil {
		return nil, err
	}
	proof.Parents = parentProofs
	return proof, nil
}

func (p *Prover) ProveSpace(challenges map[int64]int64) (*SpaceProof, error) {
	spaceProot := &SpaceProof{
		Proofs: make([]MerkelProofItem, len(challenges)),
		Roots:  make([][]byte, len(challenges)),
		Index:  make([]int64, len(challenges)),
		Wits:   make([][]byte, len(challenges)),
	}
	count := 0
	for k, v := range challenges {
		dir := p.FilePath[k-1]
		data, err := util.ReadProofFile(
			path.Join(dir, fmt.Sprintf("%s-%d", graph.LAYER_NAME, p.graph.K)),
			int(p.graph.N),
			graph.HashSize,
		)
		if err != nil {
			return nil, errors.Wrap(err, "prove space error")
		}
		index := v % p.graph.N
		mt, err := tree.CalculateMerkelTree(data)
		if err != nil {
			return nil, errors.Wrap(err, "prove space error")
		}
		paths, locs, err := tree.CalculateTreePathWitTree(mt, data[index])
		if err != nil {
			return nil, errors.Wrap(err, "prove space error")
		}
		spaceProot.Proofs[count] = MerkelProofItem{
			Index: v,
			Label: data[index],
			Paths: paths,
			Locs:  locs,
		}
		spaceProot.Roots[count] = mt.MerkleRoot()
		spaceProot.Index[count] = k
		spaceProot.Wits[count] = p.AccManager.Witness[k-1]
		count++
	}
	return spaceProot, nil
}

func (p *Prover) ProveDeletion() (*DeletionProof, error) {
	if p.Count == 0 {
		err := errors.New("empty proofs")
		return nil, errors.Wrap(err, "prove deletion error")
	}
	dproof := &DeletionProof{}
	dproof.Acc = p.AccManager.Witness[p.Count-1]
	fdir := p.FilePath[p.Count-1]
	data, err := util.ReadProofFile(
		path.Join(fdir, graph.COMMIT_FILE),
		int(p.graph.K+2), graph.HashSize,
	)
	if err != nil {
		return nil, errors.Wrap(err, "prove deletion error")
	}
	dproof.Root = data[p.graph.K]
	p.AccManager.DeleteOneMember()
	dir := p.FilePath[p.Count-1]
	if err := util.DeleteDir(dir); err != nil {
		return nil, errors.Wrap(err, "prove deletion error")
	}
	p.FilePath = p.FilePath[:p.Count-1]
	p.Count -= 1
	return dproof, nil
}
