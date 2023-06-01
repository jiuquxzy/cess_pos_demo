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

// ProofNode denote Prover
type ProofNode struct {
	ID        []byte   // Prover ID(generally, use AccountID)
	CommitBuf [][]byte //buffer for all layer MHT proofs of one commit
	Commits   [][]byte //Prover's Commit proofs
	Acc       []byte   //Prover's accumulator
	Count     int64    // Idle file proofs' counter
}

type Verifier struct {
	K, N, D int64                 //Graph params,graph size=(K+1)*N,D is in-degree of each node
	Key     acc.RsaKey            //accumulator public key
	Nodes   map[string]*ProofNode //Provers
}

func NewVerifier(key acc.RsaKey, k, n, d int64) *Verifier {
	return &Verifier{
		K: k, N: n, D: d,
		Key:   key,
		Nodes: make(map[string]*ProofNode),
	}
}

func (v *Verifier) AddParams(param ProofParams) bool {
	if v.K != param.K || v.N != param.N || v.D != param.D {
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
		v.Nodes[id] = &ProofNode{
			ID: ID,
		}
		pNode = v.Nodes[id]
	} else if !bytes.Equal(pNode.ID, ID) {
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
	//Randomly select one node from each layer in the graph as a challenge
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
	//Randomly select several nodes from idle files as random challenges
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
		//verify MHT Proof
		hash := graph.GetHash(proof.Node.Label)
		ok = tree.VerifyTreePath(proof.Node.Paths, proof.Node.Locs, hash, root)
		if !ok {
			log.Println("verify node tree path error")
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
			hash := graph.GetHash(p.Label)
			if p.Index >= layer*v.N {
				root = node.CommitBuf[layer]
			} else {
				root = node.CommitBuf[layer-1]
			}
			//verify parent MHT Proof
			ok = tree.VerifyTreePath(p.Paths, p.Locs, hash, root)
			if !ok {
				log.Println("verify parent tree path error")
				return false, nil
			}
			label = append(label, p.Label...)
		}
		//Verify each node pebbled from all parents
		hash = graph.GetHash(label)
		if !bytes.Equal(hash, proof.Node.Label) {
			log.Println("verify node label error")
			return false, nil
		}
	}
	//verify acc
	if node.Acc == nil {
		node.Acc = v.Key.G.Bytes()
	}
	elem := append(ID, util.Int64Bytes(node.Count+1)...)
	elem = append(elem, node.CommitBuf[v.K]...)
	hash := graph.GetHash(elem)
	if !acc.Verify(v.Key, commitProof.ACC, hash, node.Acc) {
		log.Println("verify acc error")
		return false, nil
	}
	//update prover params
	node.Acc = commitProof.ACC
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
		//verify MHT Proof
		hash := graph.GetHash(spaceProof.Proofs[i].Label)
		if !tree.VerifyTreePath(spaceProof.Proofs[i].Paths, spaceProof.Proofs[i].Locs,
			hash, spaceProof.Roots[i]) {
			return false, nil
		}
		//verify MHT root
		if !bytes.Equal(node.Commits[spaceProof.Index[i]-1], spaceProof.Roots[i]) {
			return false, nil
		}
		//verify acc
		label := append(ID, util.Int64Bytes(spaceProof.Index[i])...)
		label = append(label, spaceProof.Roots[i]...)
		hash = graph.GetHash(label)
		if !acc.Verify(v.Key, node.Acc, hash, spaceProof.Wits[i]) {
			return false, nil
		}
	}
	return true, nil
}

func (v *Verifier) VerifyDeletion(ID []byte, proof *DeletionProof) (bool, error) {
	node, ok := v.Nodes[hex.EncodeToString(ID)]
	if !ok {
		err := errors.New("bad prover ID")
		return false, errors.Wrap(err, "verify space proof error")
	}
	//verify acc
	label := append(ID, util.Int64Bytes(node.Count)...)
	label = append(label, proof.Root...)
	hash := graph.GetHash(label)
	if !acc.Verify(v.Key, node.Acc, hash, proof.Acc) {
		return false, nil
	}
	node.Commits = node.Commits[:node.Count-1]
	node.Count -= 1
	node.Acc = proof.Acc
	return true, nil
}
