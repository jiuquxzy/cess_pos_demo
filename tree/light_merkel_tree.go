package tree

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
	"math"
)

type LightMHT []byte

type PathProof struct {
	Locs []byte
	Path [][]byte
}

func CalcLightMmtWitBytes(data []byte, size int) LightMHT {
	var mht LightMHT
	lens := len(data)
	if lens%size != 0 {
		return mht
	}
	mht = make(LightMHT, lens)
	hash := NewHash(size)
	for i := 0; i < lens/size; i++ {
		hash.Reset()
		hash.Write(data[i*size : (i+1)*size])
		copy(mht[i*size:(i+1)*size], hash.Sum(nil))
	}
	p := lens / 2
	src := mht[:]
	for i := 0; i < int(math.Log2(float64(lens/size)))+1; i++ {
		num := lens / (1 << (i + 1))
		target := mht[p : p+num]
		for j, k := num/size-1, num*2/size-2; j >= 0 && k >= 0; j, k = j-1, k-2 {
			hash.Reset()
			hash.Write(src[k*size : (k+2)*size])
			copy(target[j*size:(j+1)*size], hash.Sum(nil))
		}
		p = p / 2
		src = target
	}
	return mht
}

func (mht LightMHT) GetRoot(size int) []byte {
	if len(mht) < size*2 {
		return nil
	}
	root := make([]byte, size)
	copy(root, mht[size:size*2])
	return root
}

func (mht LightMHT) GetPathProof(data []byte, index, size int) (PathProof, error) {
	if len(mht) != len(data) {
		return PathProof{}, errors.New("error data")
	}
	lens := int(math.Log2(float64(len(data) / size)))
	proof := PathProof{
		Locs: make([]byte, lens),
		Path: make([][]byte, lens),
	}
	var (
		loc byte
		d   []byte
	)
	hash := NewHash(size)
	num, p := len(data), len(mht)
	for i := 0; i < lens; i++ {
		if (index+1)%2 == 0 {
			loc = 0
			d = data[(index-1)*size : index*size]
		} else {
			loc = 1
			d = data[(index+1)*size : (index+2)*size]
		}
		if i == 0 {
			hash.Reset()
			hash.Write(d)
			proof.Path[i] = hash.Sum(nil)
		} else {
			proof.Path[i] = d
		}
		proof.Locs[i] = loc
		num, index = num/2, index/2
		p -= num
		data = mht[p : p+num]
	}
	return proof, nil
}

func VerifyPathProof(root, data []byte, proof PathProof) bool {
	if len(proof.Locs) != len(proof.Path) {
		return false
	}
	hash := NewHash(len(root))
	hash.Write(data)
	data = hash.Sum(nil)
	if len(data) != len(root) {
		return false
	}
	for i := 0; i < len(proof.Path); i++ {
		hash.Reset()
		if proof.Locs[i] == 0 {
			hash.Write(append(proof.Path[i][:], data...))
		} else {
			hash.Write(append(data, proof.Path[i]...))
		}
		data = hash.Sum(nil)
	}
	return bytes.Equal(root, data)
}

func NewHash(size int) hash.Hash {
	switch size {
	case 32:
		return sha256.New()
	case 64:
		return sha512.New()
	default:
		return sha512.New()
	}
}
