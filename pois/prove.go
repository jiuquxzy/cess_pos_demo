package pois

import (
	"bytes"
	"cess_pos_demo/acc"
	"cess_pos_demo/expanders"
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"fmt"
	"log"
	"path"
	"sync"
	"sync/atomic"

	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
)

var (
	prover               *Prover
	MaxCommitProofThread = 16
	MaxSpaceProofThread  = 64
	AvailableSpace       int64 //unit(MiB)
)

type Prover struct {
	Expanders  *expanders.Expanders
	Count      int64
	Commited   atomic.Int64
	Added      int64
	Generated  atomic.Int64
	FilePath   string
	rw         sync.RWMutex
	delete     atomic.Bool
	workdone   atomic.Bool
	ID         []byte
	cmdCh      chan<- int64
	resCh      <-chan bool
	AccManager acc.AccHandle
}

type MhtProof struct {
	Index expanders.NodeType `json:"index"`
	Label []byte             `json:"label"`
	Paths [][]byte           `json:"paths"`
	Locs  []byte             `json:"locs"`
}

type Commit struct {
	FileIndex int64    `json:"file_index"`
	Roots     [][]byte `json:"roots"`
}

type CommitProof struct {
	Node    *MhtProof   `json:"node"`
	Parents []*MhtProof `json:"parents"`
}

type AccProof struct {
	Indexs    []int64          `json:"indexs"`
	Labels    [][]byte         `json:"labels"`
	WitChains *acc.WitnessNode `json:"wit_chains"`
	AccPath   [][]byte         `json:"acc_path"`
}

type SpaceProof struct {
	Proofs    []*MhtProof        `json:"proofs"`
	Roots     [][]byte           `json:"roots"`
	WitChains []*acc.WitnessNode `json:"wit_chains"`
}

type DeletionProof struct {
	Roots    [][]byte         `json:"roots"`
	WitChain *acc.WitnessNode `json:"wit_chain"`
	AccPath  [][]byte         `json:"acc_path"`
}

func InitProver(k, n, d int64, ID []byte, filePath, accPath string, key acc.RsaKey) error {
	if prover != nil {
		return nil
	}
	prover = &Prover{
		ID:       ID,
		FilePath: filePath,
	}
	prover.Expanders = expanders.ConstructStackedExpanders(k, n, d)
	var err error
	prover.AccManager, err = acc.NewMutiLevelAcc(accPath, key)
	return errors.Wrap(err, "init prover error")
}

func SetAvailableSpace(size int64) {
	AvailableSpace = size
}

func GetProver() *Prover {
	return prover
}

func (p *Prover) GenerateFile(num int64) bool {
	if num <= 0 {
		return false
	}
	if !p.workdone.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		for i := p.Added + 1; i <= p.Added+num; i++ {
			p.cmdCh <- i
		}
		p.Added += num
		p.workdone.Store(false)
	}()
	return true
}

func (p *Prover) UpdateCount(num int64) bool {
	p.rw.Lock()
	defer p.rw.Unlock()
	p.Count += num
	p.organizeFiles(num)
	return true
}

func (p *Prover) RunIdleFileGenerationServer(threadNum int) {
	expanders.InitLabelsPool(p.Expanders.N, int64(expanders.HashSize))
	p.cmdCh, p.resCh = expanders.IdleFileGenerationServer(p.Expanders,
		p.ID, p.FilePath, threadNum)
	go func() {
		for res := range prover.resCh {
			if !res {
				return
			}
			p.Generated.Add(1)
		}
	}()
}

// GetCommits can not run concurrently!
func (p *Prover) GetCommits(num int64) ([]Commit, error) {
	var err error
	if p.delete.Load() {
		return nil, nil
	}
	fileNum := p.Generated.Load()
	commited := p.Commited.Load()
	if fileNum-commited <= 0 {
		err = errors.New("no idle file generated")
		return nil, errors.Wrap(err, "get commits error")
	}
	if fileNum-commited < num {
		num = fileNum - commited
	}
	commits := make([]Commit, num)
	for i := int64(1); i <= num; i++ {
		commits[i-1].FileIndex = commited + i
		name := path.Join(
			p.FilePath,
			fmt.Sprintf("%s-%d", expanders.IDLE_DIR_NAME, commited+i),
			expanders.COMMIT_FILE,
		)
		commits[i-1].Roots, err = util.ReadProofFile(
			name, int(p.Expanders.K+2), expanders.HashSize)
		if err != nil {
			return nil, errors.Wrap(err, "get commits error")
		}
	}
	if !p.Commited.CompareAndSwap(commited, commited+num) {
		err = errors.New("commit counter has been modified")
		return nil, errors.Wrap(err, "get commits error")
	}
	return commits, nil
}

func (p *Prover) CommitRollback(past int64) bool {
	now := p.Commited.Load()
	return p.Commited.CompareAndSwap(now, past)
}

// ProveCommit prove commits no more than MaxCommitProofThread
func (p *Prover) ProveCommit(challenges [][]int64) ([][]CommitProof, error) {
	var (
		threads int
		err     error
	)
	lens := len(challenges)
	if lens < MaxCommitProofThread {
		threads = lens
	} else {
		threads = MaxCommitProofThread
	}
	proofSet := make([][]CommitProof, lens)

	wg := sync.WaitGroup{}
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		idx := i
		ants.Submit(func() {
			defer wg.Done()
			proofs, e := p.proveCommit(challenges[idx])
			if e != nil {
				err = e
				return
			}
			proofSet[idx] = proofs
		})
	}
	wg.Wait()
	if err != nil {
		return nil, errors.Wrap(err, "prove commits error")
	}
	return proofSet, errors.Wrap(err, "prove commits error")
}

func (p *Prover) ProveAcc(indexs []int64) (*AccProof, error) {
	var err error
	labels := make([][]byte, len(indexs))
	for i := 0; i < len(indexs); i++ {
		labels[i], err = p.ReadAndCalcFileLabel(indexs[i])
		if err != nil {
			return nil, errors.Wrap(err, "update acc and Count error")
		}
	}
	witChain, accPath, err := p.AccManager.AddElementsAndProof(labels)
	if err != nil {
		return nil, errors.Wrap(err, "update acc and Count error")
	}

	return &AccProof{
		Indexs:    indexs,
		Labels:    labels,
		WitChains: witChain,
		AccPath:   accPath,
	}, nil
}

func (p *Prover) ReadAndCalcFileLabel(Count int64) ([]byte, error) {
	name := path.Join(
		p.FilePath,
		fmt.Sprintf("%s-%d", expanders.IDLE_DIR_NAME, Count),
		expanders.COMMIT_FILE,
	)
	roots, err := util.ReadProofFile(
		name, int(p.Expanders.K+2), expanders.HashSize)
	if err != nil {
		return nil, errors.Wrap(err, "read file root hashs error")
	}
	root := roots[p.Expanders.K]
	label := append([]byte{}, p.ID...)
	label = append(label, expanders.GetBytes(Count)...)
	return expanders.GetHash(append(label, root...)), nil
}

func (p *Prover) ReadFileLabels(Count int64, buf []byte) error {
	name := path.Join(
		p.FilePath,
		fmt.Sprintf("%s-%d", expanders.IDLE_DIR_NAME, Count),
		fmt.Sprintf("%s-%d", expanders.LAYER_NAME, p.Expanders.K),
	)
	if err := util.ReadFileToBuf(name, buf); err != nil {
		return errors.Wrap(err, "read file labels error")
	}
	return nil
}

func (p *Prover) proveCommit(challenge []int64) ([]CommitProof, error) {
	var (
		err   error
		index expanders.NodeType
	)
	proofs := make([]CommitProof, len(challenge)-1)
	fdir := path.Join(p.FilePath, fmt.Sprintf("%s-%d", expanders.IDLE_DIR_NAME, challenge[0]))
	proofs[0], err = p.generateCommitProof(fdir, challenge[0], challenge[1])
	if err != nil {
		return nil, errors.Wrap(err, "prove one file commit error")
	}
	for i := 2; i < len(challenge); i++ {
		index = proofs[i-2].Parents[challenge[i]].Index
		proofs[i-1], err = p.generateCommitProof(fdir, challenge[0], int64(index))
		if err != nil {
			return nil, errors.Wrap(err, "prove one file commit error")
		}
	}
	return proofs, nil
}

func (p *Prover) generateCommitProof(fdir string, count, c int64) (CommitProof, error) {
	layer := c / p.Expanders.N
	if layer < 0 || layer > p.Expanders.K {
		return CommitProof{}, errors.New("generate commit proof error: bad node index")
	}

	fpath := path.Join(fdir, fmt.Sprintf("%s-%d", expanders.LAYER_NAME, layer))
	log.Println("path", fpath)
	data := expanders.GetPool().Get().(*[]byte)
	defer expanders.GetPool().Put(data)
	if err := util.ReadFileToBuf(fpath, *data); err != nil {
		return CommitProof{}, errors.Wrap(err, "generate commit proof error")
	}

	var nodeTree, parentTree *tree.LightMHT
	index := c % p.Expanders.N
	nodeTree = tree.CalcLightMhtWithBytes(*data, expanders.HashSize, true)
	defer tree.RecoveryMht(nodeTree)
	pathProof, err := nodeTree.GetPathProof(*data, int(index), expanders.HashSize)
	if err != nil {
		return CommitProof{}, errors.Wrap(err, "generate commit proof error")
	}
	label := make([]byte, expanders.HashSize)
	copy(label, (*data)[index*int64(expanders.HashSize):(index+1)*int64(expanders.HashSize)])
	proof := CommitProof{
		Node: &MhtProof{
			Index: expanders.NodeType(c),
			Label: label,
			Paths: pathProof.Path,
			Locs:  pathProof.Locs,
		},
	}

	if layer == 0 {
		return proof, nil
	}
	node := expanders.NewNode(expanders.NodeType(c))
	node.Parents = make([]expanders.NodeType, 0, p.Expanders.D+1)
	expanders.CalcParents(p.Expanders, node, p.ID, count)
	fpath = path.Join(fdir, fmt.Sprintf("%s-%d", expanders.LAYER_NAME, layer-1))
	pdata := expanders.GetPool().Get().(*[]byte)
	defer expanders.GetPool().Put(pdata)
	err = util.ReadFileToBuf(fpath, *pdata)
	if err != nil {
		return proof, errors.Wrap(err, "generate commit proof error")
	}

	parentTree = tree.CalcLightMhtWithBytes(*pdata, expanders.HashSize, true)
	defer tree.RecoveryMht(parentTree)
	lens := len(node.Parents)
	parentProofs := make([]*MhtProof, lens)
	wg := sync.WaitGroup{}
	wg.Add(lens)
	for i := 0; i < lens; i++ {
		idx := i
		ants.Submit(func() {
			defer wg.Done()
			index := int64(node.Parents[idx]) % p.Expanders.N
			label := make([]byte, expanders.HashSize)
			var (
				pathProof tree.PathProof
				e         error
			)
			if int64(node.Parents[idx]) >= layer*p.Expanders.N {
				copy(label, (*data)[index*int64(expanders.HashSize):(index+1)*int64(expanders.HashSize)])
				pathProof, e = nodeTree.GetPathProof(*data, int(index), expanders.HashSize)
			} else {
				copy(label, (*pdata)[index*int64(expanders.HashSize):(index+1)*int64(expanders.HashSize)])
				pathProof, e = parentTree.GetPathProof(*pdata, int(index), expanders.HashSize)
			}
			if e != nil {
				err = e
				return
			}
			parentProofs[idx] = &MhtProof{
				Index: node.Parents[idx],
				Label: label,
				Paths: pathProof.Path,
				Locs:  pathProof.Locs,
			}
		})
	}
	wg.Wait()
	if err != nil {
		return proof, err
	}
	proof.Parents = parentProofs
	//log.Println("id", string(p.ID), "count", count, "index", c)
	labels := append([]byte{}, p.ID...)
	labels = append(labels, expanders.GetBytes(count)...)
	labels = append(labels, expanders.GetBytes(proof.Node.Index)...)
	for i := 0; i < len(proof.Parents); i++ {
		labels = append(labels, proof.Parents[i].Label...)
	}
	log.Println("label result", bytes.Equal(expanders.GetHash(labels), proof.Node.Label))
	return proof, nil
}

func (p *Prover) ProveSpace(challenges [][]int64) (*SpaceProof, error) {
	var err error
	lens := len(challenges)
	if lens <= 0 {
		err := errors.New("bad challenge length")
		return nil, errors.Wrap(err, "prove space error")
	}
	proof := &SpaceProof{
		Proofs:    make([]*MhtProof, lens),
		Roots:     make([][]byte, lens),
		WitChains: make([]*acc.WitnessNode, lens),
	}
	indexs := make([]int64, lens)
	data := expanders.GetPool().Get().(*[]byte)
	for i := 0; i < lens; i++ {
		if err := p.ReadFileLabels(challenges[i][0], *data); err != nil {
			return nil, errors.Wrap(err, "prove space error")
		}
		mht := tree.CalcLightMhtWithBytes(*data, expanders.HashSize, true)

		indexs[i] = challenges[i][0]
		proof.Roots[i] = mht.GetRoot(expanders.HashSize)

		idx := int(challenges[i][1]) % expanders.HashSize
		mhtProof, err := mht.GetPathProof(*data, idx, expanders.HashSize)
		if err != nil {
			return nil, errors.Wrap(err, "prove space error")
		}

		label := make([]byte, expanders.HashSize)
		copy(label, (*data)[idx*expanders.HashSize:(idx+1)*expanders.HashSize])

		proof.Proofs[i] = &MhtProof{
			Paths: mhtProof.Path,
			Locs:  mhtProof.Locs,
			Index: expanders.NodeType(challenges[i][1]),
			Label: label,
		}

		tree.RecoveryMht(mht)
	}
	expanders.GetPool().Put(data)
	proof.WitChains, err = p.AccManager.GetWitnessChains(indexs)
	if err != nil {
		return nil, errors.Wrap(err, "prove space error")
	}
	return proof, nil
}

func (p *Prover) ProveDeletion(num int64) (chan *DeletionProof, chan error) {
	ch := make(chan *DeletionProof, 1)
	Err := make(chan error, 1)
	if num <= 0 {
		err := errors.New("bad file number")
		Err <- errors.Wrap(err, "prove deletion error")
		return ch, Err
	}
	go func() {
		p.delete.Store(true)
		defer p.delete.Store(false)
		var tmp int64
		roots := make([][]byte, num)
		for {
			commited := p.Commited.Load()
			p.rw.RLock()
			if p.Count == commited {
				p.rw.RUnlock()
				break
			}
			p.rw.RUnlock()
		}
		size := p.Expanders.N * num / (1024 * 1024)
		if size < AvailableSpace {
			ch <- nil
			Err <- nil
			return
		}
		p.rw.Lock()
		if p.Count < num {
			p.rw.Unlock()
			ch <- nil
			err := errors.New("insufficient operating space")
			Err <- errors.Wrap(err, "prove deletion error")
			return
		}
		p.Count = p.Count - num
		tmp = p.Count
		p.rw.Unlock()
		data := expanders.GetPool().Get().(*[]byte)
		for i := int64(1); i <= num; i++ {
			dir := path.Join(p.FilePath,
				fmt.Sprintf("%s-%d", expanders.IDLE_DIR_NAME, tmp+i))
			if err := p.ReadFileLabels(tmp+i, *data); err != nil {
				Err <- errors.Wrap(err, "prove deletion error")
				return
			}
			mht := tree.CalcLightMhtWithBytes(*data, expanders.HashSize, true)
			roots[i-1] = mht.GetRoot(expanders.HashSize)
			tree.RecoveryMht(mht)
			if err := util.DeleteDir(dir); err != nil {
				Err <- errors.Wrap(err, "prove deletion error")
				return
			}
		}
		wits, accs, err := p.AccManager.DeleteElementsAndProof(int(num))
		if err != nil {
			Err <- errors.Wrap(err, "prove deletion error")
			return
		}
		ch <- &DeletionProof{
			Roots:    roots,
			WitChain: wits,
			AccPath:  accs,
		}
	}()
	return nil, nil
}

func (p *Prover) organizeFiles(num int64) error {
	for i := p.Count + 1; i <= p.Count+num; i++ {
		dir := path.Join(p.FilePath,
			fmt.Sprintf("%s-%d", expanders.IDLE_DIR_NAME, i))
		for j := 0; j < int(p.Expanders.K); j++ {
			name := path.Join(dir, fmt.Sprintf("%s-%d", expanders.LAYER_NAME, j))
			if err := util.DeleteFile(name); err != nil {
				return err
			}
		}
		name := path.Join(dir, expanders.COMMIT_FILE)
		if err := util.DeleteFile(name); err != nil {
			return err
		}
	}
	return nil
}
