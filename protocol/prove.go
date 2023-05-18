package protocol

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/graph"
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"fmt"
	"path"

	"github.com/pkg/errors"
)

type Prover struct {
	graph      *graph.StackedExpanders
	Count      int64
	ID         []byte
	FilePath   []string
	AccManager *acc.AccManager
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

func NewProver(key acc.RsaKey, id []byte) *Prover {
	return &Prover{
		ID: id,
		AccManager: &acc.AccManager{
			Key: key,
		},
	}
}

func (p *Prover) NewGraph(path string, n int64, k int64, d int64, localize bool) error {
	var err error
	p.graph, err = graph.ConstructStackedExpanders(path, n, k, d, localize)
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

func (p *Prover) ReadCommitProof(fidx int) ([][]byte, error) {
	if fidx < 0 || fidx > len(p.FilePath) {
		return nil, errors.New("create commit proof error index out of range")
	}
	path := p.FilePath[fidx]
	data, err := util.ReadProofFile(path, int(p.graph.K), graph.HashSize)
	if err != nil {
		return nil, errors.Wrap(err, "create commit proof error")
	}
	return data, nil
}

func (p *Prover) ProveCommit(fdir string, challenges map[int64]struct{}) (*CommitProof, error) {
	proofs := make([]CommitProofItem, len(challenges))
	i := 0
	for k := range challenges {
		proof, err := p.GenerateCommitProof(fdir, k)
		if err != nil {
			return nil, errors.Wrap(err, "prove commit error")
		}
		proofs[i] = *proof
		i++
	}
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
	var accProof []byte
	if p.AccManager.Acc == nil {
		accProof = acc.GenerateAcc(p.AccManager.Key, [][]byte{hash})
		p.AccManager.Acc = accProof
		p.AccManager.Elems = append(p.AccManager.Elems, accProof)
	} else {
		p.AccManager.AddMember(hash)
		accProof = p.AccManager.Acc
	}
	return &CommitProof{
		Proofs: proofs,
		ACC:    accProof,
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
	index := c % p.graph.N
	treePath, locs, err := tree.CalculateTreePath(data, int(index))
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	if layer == 0 {
		return &CommitProofItem{
			Node: MerkelProofItem{
				Index: c,
				Label: data[index],
				Paths: treePath,
				Locs:  locs,
			},
		}, nil
	}
	fpath = path.Join(fdir, fmt.Sprintf("%s-%d", graph.LAYER_NAME, layer-1))
	parents, err := p.graph.GetParents(c)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	data, err = util.ReadProofFile(fpath, int(p.graph.N), graph.HashSize)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	parentsTree, err := tree.CalculateMerkelTree(data)
	if err != nil {
		return nil, errors.Wrap(err, "generate commit proof error")
	}
	list := graph.Sort(parents)
	parentProofs := make([]MerkelProofItem, len(list))
	for i := 0; i < len(list); i++ {
		index := c % p.graph.N
		treePath, locs, err := tree.CalculateTreePathWitTree(parentsTree, data[index])
		if err != nil {
			return nil, errors.Wrap(err, "generate commit proof error")
		}
		parentProofs[i] = MerkelProofItem{
			Index: list[i],
			Label: data[index],
			Paths: treePath,
			Locs:  locs,
		}
	}
	return &CommitProofItem{
		Node: MerkelProofItem{
			Index: c,
			Label: data[index],
			Paths: treePath,
			Locs:  locs,
		},
		Parents: parentProofs,
	}, nil
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
