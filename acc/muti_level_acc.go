package acc

import (
	"bytes"
	"cess_pos_demo/util"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

const (
	DEFAULT_PATH      = "./acc/"
	DEFAULT_DIR_PERM  = 0777
	DEFAULT_ELEMS_NUM = 1024
	DEFAULT_LEVEL     = 3
	DEFAULT_NAME      = "sub-acc"
)

type AccNode struct {
	Value    []byte
	Children []*AccNode
	Len      int
	Wit      []byte
}

type AccData struct {
	Values [][]byte `json:"values"`
	Wits   [][]byte `json:"wits"`
}

type WitnessNode struct {
	Elem []byte       `json:"elem"`
	Wit  []byte       `json:"wit"`
	Acc  *WitnessNode `json:"acc"`
}

type MutiLevelAcc struct {
	Accs      *AccNode
	Key       RsaKey
	ElemNums  int
	CurrCount int
	Curr      *AccNode
	Parent    *AccNode
	mu        *sync.Mutex
	isUpdate  bool
	snapshot  *MutiLevelAcc
	FilePath  string
}

func NewMutiLevelAcc(path string, key RsaKey) (*MutiLevelAcc, error) {
	if path == "" {
		path = DEFAULT_PATH
	}
	if _, err := os.Stat(path); err != nil {
		err := os.MkdirAll(path, DEFAULT_DIR_PERM)
		if err != nil {
			return nil, err
		}
	}
	acc := &AccNode{
		Value: key.G.Bytes(),
	}
	return &MutiLevelAcc{
		Accs:     acc,
		Key:      key,
		mu:       new(sync.Mutex),
		FilePath: path,
	}, nil
}

// GetSnapshot get ACC's snapshot if it is updating,else create and return new snapshot.
// Snapshot ensure that every space proof does not read an incomplete accumulator
func (acc *MutiLevelAcc) GetSnapshot() *MutiLevelAcc {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if acc.isUpdate && acc.snapshot != nil {
		return acc.snapshot
	} else {
		acc.createSnapshot()
		return acc.snapshot
	}
}

func (acc *MutiLevelAcc) copy(other *MutiLevelAcc) {
	if other == nil {
		return
	}
	accs := &AccNode{}
	copyAccNode(other.Accs, accs)
	acc.Accs = accs
	acc.Key = other.Key
	acc.ElemNums = other.ElemNums
	acc.CurrCount = other.CurrCount
	acc.Curr = other.Curr
	acc.Parent = other.Parent
	acc.mu = other.mu
	acc.FilePath = other.FilePath
}

func (acc *MutiLevelAcc) createSnapshot() {
	acc.snapshot = &MutiLevelAcc{}
	acc.snapshot.copy(acc)
}

func (acc *MutiLevelAcc) setUpdate(yes bool) bool {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if acc.isUpdate {
		return false
	}
	if yes {
		acc.createSnapshot()
		acc.isUpdate = true
	} else {
		acc.isUpdate = false
	}
	return true
}

func (acc *MutiLevelAcc) updateAcc(node *AccNode) {
	lens := len(node.Children)
	node.Len = lens
	if lens == 0 {
		node.Value = acc.Key.G.Bytes()
		node.Wit = nil
		return
	}
	genWitsForAccNodes(acc.Key.G, acc.Key.N, node.Children)
	last := node.Children[lens-1]
	node.Value = generateAcc(acc.Key, last.Wit, [][]byte{last.Value})
}

func (acc *MutiLevelAcc) AddElements(elems [][]byte) error {
	lens := len(elems)
	//the range of length of elems be insert is [0,1024]
	if lens <= 0 || acc.CurrCount < DEFAULT_ELEMS_NUM &&
		lens+acc.CurrCount > DEFAULT_ELEMS_NUM {
		err := errors.New("illegal number of elements")
		return errors.Wrap(err, "add elements error")
	}
	if !acc.setUpdate(true) {
		err := errors.New("update permission is occupied")
		return errors.Wrap(err, "add elements error")
	}
	defer acc.setUpdate(false)
	newAcc, err := acc.addElements(elems)
	if err != nil {
		return errors.Wrap(err, "add elements error")
	}
	acc.addSubAcc(newAcc)
	return nil
}

func (acc *MutiLevelAcc) addElements(elems [][]byte) (*AccNode, error) {
	var (
		data *AccData
		err  error
	)
	node := &AccNode{}
	if acc.CurrCount > 0 && acc.CurrCount < DEFAULT_ELEMS_NUM {
		index := acc.ElemNums / DEFAULT_ELEMS_NUM
		data, err = readAccData(DEFAULT_PATH, index)
		if err != nil {
			return nil, errors.Wrap(err, "add elements to sub acc error")
		}
		data.Values = append(data.Values, elems...)
	} else {
		data = new(AccData)
		data.Values = elems
	}
	data.Wits = generateWitness(acc.Key.G, acc.Key.N, data.Values)
	node.Len = len(data.Values)
	node.Value = generateAcc(
		acc.Key, data.Wits[node.Len-1],
		[][]byte{data.Values[node.Len-1]},
	)
	index := (acc.ElemNums + len(elems)) / DEFAULT_ELEMS_NUM
	err = saveAccData(DEFAULT_PATH, index, data.Values, data.Wits)
	return node, errors.Wrap(err, "add elements to sub acc error")
}

// addSubAccs inserts the sub acc built with new elements into the multilevel accumulator
func (acc *MutiLevelAcc) addSubAcc(subAcc *AccNode) {
	if acc.CurrCount == 0 {
		acc.Curr = subAcc
		acc.CurrCount = acc.Curr.Len
		acc.Curr.Wit = acc.Key.G.Bytes()
		acc.Parent = &AccNode{
			Value:    generateAcc(acc.Key, acc.Key.G.Bytes(), [][]byte{subAcc.Value}),
			Wit:      acc.Key.G.Bytes(),
			Children: []*AccNode{subAcc},
			Len:      1,
		}
		acc.Accs = &AccNode{
			Value:    generateAcc(acc.Key, acc.Key.G.Bytes(), [][]byte{acc.Parent.Value}),
			Children: []*AccNode{acc.Parent},
			Len:      1,
		}
		acc.ElemNums += acc.CurrCount
		return
	}
	if acc.CurrCount > 0 && acc.CurrCount < DEFAULT_ELEMS_NUM {
		acc.ElemNums += subAcc.Len - acc.CurrCount
		lens := len(acc.Parent.Children)
		acc.Parent.Children[lens-1] = subAcc
	} else if len(acc.Parent.Children)+1 <= DEFAULT_ELEMS_NUM {
		acc.ElemNums += subAcc.Len
		acc.Parent.Children = append(acc.Parent.Children, subAcc)
	} else {
		acc.ElemNums += subAcc.Len
		node := &AccNode{
			Wit:      acc.Key.G.Bytes(),
			Children: []*AccNode{subAcc},
		}
		acc.Accs.Children = append(acc.Accs.Children, node)
		acc.Parent = node
	}
	acc.Curr = subAcc
	acc.CurrCount = acc.Curr.Len
	//update sibling witness and parent acc
	acc.updateAcc(acc.Parent)
	//update parents and top acc
	acc.updateAcc(acc.Accs)
}

func (acc *MutiLevelAcc) DeleteElements(num int) error {
	if num <= 0 || acc.CurrCount > 0 && num > acc.CurrCount {
		err := errors.New("illegal number of elements")
		return errors.Wrap(err, "delete elements error")
	}
	if !acc.setUpdate(true) {
		err := errors.New("update permission is occupied")
		return errors.Wrap(err, "delet elements error")
	}
	defer acc.setUpdate(false)
	if num < acc.CurrCount {
		index := acc.ElemNums / DEFAULT_ELEMS_NUM
		data, err := readAccData(DEFAULT_PATH, index)
		if err != nil {
			return errors.Wrap(err, "delet elements error")
		}
		data.Values = data.Values[:acc.CurrCount-num]
		data.Wits = generateWitness(acc.Key.G, acc.Key.N, data.Values)
		err = saveAccData(DEFAULT_PATH, index, data.Values, data.Wits)
		if err != nil {
			return errors.Wrap(err, "delet elements error")
		}
		acc.Curr.Len = acc.CurrCount - num
		acc.CurrCount = acc.Curr.Len
		acc.Curr.Value = generateAcc(
			acc.Key, data.Wits[acc.CurrCount-1],
			[][]byte{data.Values[acc.CurrCount-1]},
		)
	} else if acc.ElemNums == num {
		index := acc.ElemNums / DEFAULT_ELEMS_NUM
		err := deleteAccData(DEFAULT_PATH, index)
		if err != nil {
			return errors.Wrap(err, "delet elements error")
		}
		acc.Curr.Len = 0
		acc.Curr.Wit = nil
		acc.Curr.Value = acc.Key.G.Bytes()
		acc.CurrCount = 0
	} else {
		index := acc.ElemNums / DEFAULT_ELEMS_NUM
		err := deleteAccData(DEFAULT_PATH, index)
		if err != nil {
			return errors.Wrap(err, "delet elements error")
		}
		acc.Parent.Children = acc.Parent.Children[:acc.Parent.Len-1]
		acc.Parent.Len -= 1
		if acc.Parent.Len == 0 && acc.Accs.Len > 1 {
			acc.Accs.Children = acc.Accs.Children[:acc.Accs.Len-1]
			acc.Accs.Len -= 1
			acc.Parent = acc.Accs.Children[acc.Accs.Len-1]
		}
		acc.Curr = acc.Parent.Children[acc.Parent.Len-1]
		acc.CurrCount = acc.Curr.Len
	}
	acc.ElemNums -= num
	//update sibling witness and parent acc
	acc.updateAcc(acc.Parent)
	//update parents and top acc
	acc.updateAcc(acc.Accs)
	return nil
}

func copyAccNode(src *AccNode, target *AccNode) {
	if src == nil || target == nil {
		return
	}
	target.Value = make([]byte, len(src.Value))
	copy(target.Value, src.Value)
	target.Children = make([]*AccNode, len(src.Children))
	for i := 0; i < len(src.Children); i++ {
		target.Children[i] = &AccNode{}
		copyAccNode(src.Children[i], target.Children[i])
	}
}

// Generate the accumulator
func generateAcc(key RsaKey, acc []byte, elems [][]byte) []byte {
	if acc == nil {
		return nil
	}
	G := new(big.Int).SetBytes(acc)
	for _, elem := range elems {
		prime := Hprime(*new(big.Int).SetBytes(elem))
		G.Exp(G, &prime, &key.N)
	}
	return G.Bytes()
}

func generateWitness(G, N big.Int, us [][]byte) [][]byte {
	if len(us) == 0 {
		return nil
	}
	if len(us) == 1 {
		return [][]byte{G.Bytes()}
	}
	left, right := us[:len(us)/2], us[len(us)/2:]
	g1, g2 := G, G
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for _, u := range right {
			e := Hprime(*new(big.Int).SetBytes(u))
			g1.Exp(&g1, &e, &N)
		}
	}()
	go func() {
		defer wg.Done()
		for _, u := range left {
			e := Hprime(*new(big.Int).SetBytes(u))
			g2.Exp(&g2, &e, &N)
		}
	}()
	wg.Wait()
	u1 := GenerateWitness(g1, N, left)
	u2 := GenerateWitness(g2, N, left)
	return append(u1, u2...)
}

func genWitsForAccNodes(G, N big.Int, elems []*AccNode) {
	lens := len(elems)
	if lens <= 0 {
		return
	}
	if lens == 1 {
		elems[0].Wit = G.Bytes()
		return
	}
	left, right := elems[:lens/2], elems[lens/2:]
	g1, g2 := G, G
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for _, u := range right {
			e := Hprime(*new(big.Int).SetBytes(u.Value))
			g1.Exp(&g1, &e, &N)
		}
	}()
	go func() {
		defer wg.Done()
		for _, u := range left {
			e := Hprime(*new(big.Int).SetBytes(u.Value))
			g2.Exp(&g2, &e, &N)
		}
	}()
	wg.Wait()
	genWitsForAccNodes(g1, N, left)
	genWitsForAccNodes(g2, N, left)
}

func saveAccData(dir string, index int, elems, wits [][]byte) error {
	data := AccData{
		Values: elems,
		Wits:   wits,
	}
	jbytes, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "save element data error")
	}
	fpath := path.Join(dir, fmt.Sprintf("%s-%d", DEFAULT_NAME, index))
	return util.SaveFile(fpath, jbytes)
}

func readAccData(dir string, index int) (*AccData, error) {
	fpath := path.Join(dir, fmt.Sprintf("%s-%d", DEFAULT_NAME, index))
	data, err := util.ReadFile(fpath)
	if err != nil {
		return nil, errors.Wrap(err, "read element data error")
	}
	accData := &AccData{}
	err = json.Unmarshal(data, accData)
	return accData, errors.Wrap(err, "read element data error")
}

// deleteAccData delete from the given index
func deleteAccData(dir string, last int) error {
	fs, err := os.ReadDir(dir)
	if err != nil {
		return errors.Wrap(err, "delete element data error")
	}
	for _, f := range fs {
		slice := strings.Split(f.Name(), "-")
		index, err := strconv.Atoi(slice[len(slice)-1])
		if err != nil {
			return errors.Wrap(err, "delete element data error")
		}
		if index >= last {
			util.DeleteFile(path.Join(dir, f.Name()))
		}
	}
	return nil
}

// Accumulator validation interface
func VerifyAcc(key RsaKey, acc, u, wit []byte) bool {
	e := Hprime(*new(big.Int).SetBytes(u))
	dash := new(big.Int).Exp(
		big.NewInt(0).SetBytes(wit),
		&e, &key.N,
	)
	return dash.Cmp(new(big.Int).SetBytes(acc)) == 0
}

func VerifyMutilevelAcc(key RsaKey, wits *WitnessNode, acc []byte) bool {
	for wits != nil && wits.Acc != nil {
		if !VerifyAcc(key, wits.Acc.Elem, wits.Elem, wits.Wit) {
			return false
		}
		wits = wits.Acc
	}
	if wits == nil {
		return false
	}
	return bytes.Equal(wits.Elem, acc)
}

func (acc *MutiLevelAcc) AddElementsAndProof(elems [][]byte) (*WitnessNode, [][]byte, error) {
	snapshot := acc.GetSnapshot()
	exist := &WitnessNode{Elem: acc.Accs.Value}
	err := acc.AddElements(elems)
	if err != nil {
		return nil, nil, errors.Wrap(err, "proof acc insert error")
	}
	count := 1
	for p, q := acc.Accs, snapshot.Accs; p != nil && q != nil && count < DEFAULT_LEVEL; {
		exist = &WitnessNode{Acc: exist}
		if p.Len > q.Len {
			for i := count; i < DEFAULT_LEVEL; i++ {
				exist.Elem = acc.Key.G.Bytes()
				exist.Wit = exist.Acc.Elem
				exist = &WitnessNode{Acc: exist}
			}
			break
		}
		count++
		p = p.Children[p.Len-1]
		q = q.Children[q.Len-1]
		exist.Elem = q.Value
		exist.Wit = q.Wit
	}
	return exist, [][]byte{acc.Curr.Value, acc.Parent.Value, acc.Accs.Value}, nil
}

func VerifyInsertUpdate(key RsaKey, exist *WitnessNode, elems, accs [][]byte, acc []byte) bool {
	if exist == nil || len(elems) == 0 || len(accs) < DEFAULT_LEVEL {
		return false
	}
	p := exist
	//if the condition is true, a new accumulator is inserted
	for p.Acc != nil && bytes.Equal(p.Acc.Elem, p.Wit) {
		p = p.Acc
	}
	if !VerifyMutilevelAcc(key, p, acc) {
		return false
	}
	subAcc := generateAcc(key, exist.Elem, elems)
	if !bytes.Equal(subAcc, accs[0]) {
		return false
	}
	p = exist
	count := 1
	for p != nil && p.Acc != nil {
		subAcc = generateAcc(key, p.Wit, [][]byte{accs[count-1]})
		if !bytes.Equal(subAcc, accs[count]) {
			return false
		}
		p = p.Acc
		count++
	}
	return true
}

func (acc *MutiLevelAcc) DeleteElementsAndProof(num int) (*WitnessNode, [][]byte, error) {
	exist := &WitnessNode{
		Elem: acc.Curr.Value,
		Wit:  acc.Curr.Wit,
		Acc: &WitnessNode{
			Elem: acc.Parent.Value,
			Wit:  acc.Parent.Wit,
			Acc:  &WitnessNode{Elem: acc.Accs.Value},
		},
	}
	accs := make([][]byte, DEFAULT_LEVEL)
	snapshot := acc.GetSnapshot()
	err := acc.DeleteElements(num)
	if err != nil {
		return nil, nil, errors.Wrap(err, "proof acc delete error")
	}
	count := 0
	for p, q := acc.Accs, snapshot.Accs; p != nil && q != nil && count < DEFAULT_LEVEL; {
		if p.Len < q.Len {
			for i := DEFAULT_LEVEL - 1; i >= count; i-- {
				accs[i] = acc.Key.G.Bytes()
			}
			break
		}
		count++
		accs[DEFAULT_LEVEL-count] = p.Value
		p = p.Children[p.Len-1]
		q = q.Children[q.Len-1]
	}
	return exist, accs, nil
}

func VerifyDeleteUpdate(key RsaKey, exist *WitnessNode, elems, accs [][]byte, acc []byte) bool {
	if exist == nil || len(elems) == 0 || len(accs) < DEFAULT_LEVEL {
		return false
	}
	if !VerifyMutilevelAcc(key, exist, acc) {
		return false
	}
	subAcc := generateAcc(key, accs[0], elems)
	if !bytes.Equal(subAcc, exist.Elem) {
		return false
	}
	p := exist
	count := 1
	for p != nil && p.Acc != nil {
		if !bytes.Equal(accs[count-1], key.G.Bytes()) {
			subAcc = generateAcc(key, p.Wit, [][]byte{accs[count-1]})
		} else {
			subAcc = p.Wit
		}
		if !bytes.Equal(subAcc, accs[count]) {
			return false
		}
		p = p.Acc
		count++
	}
	return false
}
