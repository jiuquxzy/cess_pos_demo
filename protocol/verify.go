package protocol

import (
	"bytes"
	"cess_pos_demo/acc"
	"cess_pos_demo/graph"
	"cess_pos_demo/tree"
	"cess_pos_demo/util"
	"crypto/rand"
	"encoding/hex"
	"log"
	"math/big"

	"github.com/pkg/errors"
)

type ProofNode struct {
	ID        []byte
	CommitBuf [][]byte
	Commits   [][]byte
	Acc       []byte
	Count     int64
}

type Verifier struct {
	K, N, D int64
	Key     acc.RsaKey
	Nodes   map[string]*ProofNode
	Seed    []byte
}

func NewVerifier(key acc.RsaKey, k, n, d int64) *Verifier {
	return &Verifier{
		K: k, N: n, D: d,
		Key:   key,
		Nodes: make(map[string]*ProofNode),
	}
}

func (v *Verifier) AddParams(param ProofParams) bool {
	if v.K != param.K || v.N != param.N || v.N != param.D {
		return false
	}
	id := hex.EncodeToString(param.ID)
	v.Nodes[id] = &ProofNode{ID: param.ID}
	return true
}

func (v *Verifier) ReceiveCommits(ID []byte, commits [][]byte) bool {
	id := hex.EncodeToString(ID)
	pNode, ok := v.Nodes[id]
	if !ok {
		return false
	}
	if !bytes.Equal(pNode.ID, ID) {
		return false
	}
	if len(commits) != int(v.K+2) {
		return false
	}
	root, err := tree.CalculateMerkelTreeRoot(commits[:v.K+1])
	if err != nil {
		log.Println("receive commits error", err)
		return false
	}
	if !bytes.Equal(root, commits[v.K+1]) {
		return false
	}
	pNode.CommitBuf = commits
	return true
}

func (v *Verifier) CommitChallenges() (map[int64]struct{}, error) {
	challenges := make(map[int64]struct{})
	for i := int64(0); i <= v.K; i++ {
		r, err := rand.Int(rand.Reader, new(big.Int).SetInt64(v.N))
		if err != nil {
			return nil, errors.Wrap(err, "generate commit challenges error")
		}
		challenges[r.Int64()+i*v.N] = struct{}{}
	}
	return challenges, nil
}

func (v *Verifier) SpaceChallenges(count, param int64) (map[int64]int64, error) {
	challenges := make(map[int64]int64)
	for i := int64(0); i < param; i++ {
		for {
			r1, err := rand.Int(rand.Reader, new(big.Int).SetInt64(count))
			if err != nil {
				return nil, errors.Wrap(err, "generate commit challenges error")
			}
			//range [1,Count]
			r1 = r1.Add(r1, big.NewInt(1))
			if _, ok := challenges[r1.Int64()]; ok {
				continue
			}
			r2, err := rand.Int(rand.Reader, new(big.Int).SetInt64(v.N))
			if err != nil {
				return nil, errors.Wrap(err, "generate commit challenges error")
			}
			challenges[r1.Int64()] = r2.Int64()
			break
		}
	}
	return challenges, nil
}

func (v *Verifier) VerifyCommit(ID []byte, chals map[int64]struct{}, commitProof *CommitProof) (bool, error) {
	//node is prover
	node, ok := v.Nodes[hex.EncodeToString(ID)]
	if !ok {
		err := errors.New("bad prover ID")
		return false, errors.Wrap(err, "verify commit proof error")
	}
	//
	if len(chals) != len(commitProof.Proofs) {
		err := errors.New("proofs length error")
		return false, errors.Wrap(err, "verify commit proof error")
	}
	//verify challenge nodes
	for i := 0; i < len(commitProof.Proofs); i++ {
		proof := commitProof.Proofs[i]
		if _, ok := chals[proof.Node.Index]; !ok {
			err := errors.New("bad graph node index")
			return false, errors.Wrap(err, "verify commit proof error")
		}
		layer := proof.Node.Index / v.N
		root := node.CommitBuf[layer]
		ok = tree.VerifyTreePath(proof.Node.Paths, proof.Node.Locs, proof.Node.Label, root)
		if !ok {
			return false, nil
		}
		if len(proof.Parents) == 0 {
			continue
		}
		if len(proof.Parents) < int(v.D) {
			err := errors.New("parents number < in-degree")
			return false, errors.Wrap(err, "verify commit proof error")
		}
		label := append(ID, util.Int64Bytes(node.Count+1)...)
		label = append(label, util.Int64Bytes(proof.Node.Index)...)
		for _, p := range proof.Parents {
			root := node.CommitBuf[layer-1]
			ok = tree.VerifyTreePath(p.Paths, p.Locs, p.Label, root)
			if !ok {
				return false, nil
			}
			label = append(label, p.Label...)
		}
		hash := graph.GetHash(label)
		if !bytes.Equal(hash, proof.Node.Label) {
			return false, nil
		}
	}
	//verify acc
	if node.Acc == nil {
		node.Acc = commitProof.ACC
	} else {
		elem := append(ID, util.Int64Bytes(node.Count+1)...)
		elem = append(elem, node.CommitBuf[v.K]...)
		hash := graph.GetHash(elem)
		if !acc.Verify(v.Key, commitProof.ACC, hash, node.Acc) {
			return false, nil
		}
	}
	node.Commits = append(node.Commits, node.CommitBuf[v.K])
	node.Count += 1
	return true, nil
}

func (v *Verifier) VerifySpace(ID []byte, chals map[int64]int64, spaceProof *SpaceProof) (bool, error) {
	//node is prover
	node, ok := v.Nodes[hex.EncodeToString(ID)]
	if !ok {
		err := errors.New("bad prover ID")
		return false, errors.Wrap(err, "verify space proof error")
	}
	if len(spaceProof.Proofs) != len(chals) {
		return false, errors.Wrap(errors.New("proofs length error"), "verify space proof error")
	}
	for i := 0; i < len(spaceProof.Index); i++ {
		if idx, ok := chals[spaceProof.Index[i]]; !ok {
			err := errors.New("bad count index")
			return false, errors.Wrap(err, "verify space proof error")
		} else if idx != spaceProof.Proofs[i].Index {
			err := errors.New("bad graph node index")
			return false, errors.Wrap(err, "verify space proof error")
		}
		if !tree.VerifyTreePath(spaceProof.Proofs[i].Paths, spaceProof.Proofs[i].Locs,
			spaceProof.Proofs[i].Label, spaceProof.Roots[i]) {
			return false, nil
		}
		if !bytes.Equal(node.Commits[spaceProof.Index[i]-1], spaceProof.Roots[i]) {
			return false, nil
		}
		label := append(ID, util.Int64Bytes(spaceProof.Index[i])...)
		label = append(label, spaceProof.Roots[i]...)
		hash := graph.GetHash(label)
		if !acc.Verify(v.Key, node.Acc, hash, spaceProof.Wits[i]) {
			return false, nil
		}
	}
	return true, nil
}
