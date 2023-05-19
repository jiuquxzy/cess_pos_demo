package acc

import "math/big"

type AccManager struct {
	Key     RsaKey
	Acc     []byte
	Elems   [][]byte
	Witness [][]byte
}

func AddMember(key RsaKey, acc, u []byte) []byte {
	e := Hprime(*new(big.Int).SetBytes(u))
	preAcc := new(big.Int).SetBytes(acc)
	newAcc := new(big.Int).Exp(preAcc, &e, &key.N)
	return newAcc.Bytes()
}

func NewAccManager(key RsaKey) *AccManager {
	return &AccManager{
		Key:   key,
		Acc:   key.G.Bytes(),
		Elems: [][]byte{},
	}
}

func (m *AccManager) AddMember(u []byte) {
	e := Hprime(*new(big.Int).SetBytes(u))
	preAcc := new(big.Int).SetBytes(m.Acc)
	newACC := new(big.Int).Exp(preAcc, &e, &m.Key.N)
	m.Acc = newACC.Bytes()
	m.Elems = append(m.Elems, u)
	m.Witness = GenerateWitness(m.Key.G, m.Key.N, m.Elems)
}

func (m *AccManager) DeleteOneMember() []byte {
	if len(m.Elems) < 1 {
		return nil
	}
	if len(m.Elems) == 1 {
		e := m.Elems[0]
		m.Acc = nil
		m.Elems = nil
		return e
	}
	e := m.Elems[len(m.Elems)-1]
	m.Elems = m.Elems[:len(m.Elems)-1]
	m.Acc = GenerateAcc(m.Key, m.Elems)
	m.Witness = GenerateWitness(m.Key.G, m.Key.N, m.Elems)
	return e
}
