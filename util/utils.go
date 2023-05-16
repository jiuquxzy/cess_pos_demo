package util

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	r "math/rand"
	"os"
	"strings"

	"github.com/pkg/errors"
)

var CHARS = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
	"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}

func Int64Bytes(i int64) []byte {
	buf := make([]byte, 8)
	binary.PutVarint(buf, i)
	return buf
}

func Rand(max int64) int64 {
	buf := make([]byte, 8)
	rand.Read(buf)
	i, _ := binary.Varint(buf)
	if i < 0 {
		i *= -1
	}
	i %= max
	return i
}

func SaveProofFile(path string, data [][]byte) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "save proof file error")
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, d := range data {
		n, err := writer.Write(d)
		if err != nil || n != len(d) {
			err := errors.New(fmt.Sprint("write label error", err))
			return errors.Wrap(err, "write proof file error")
		}
	}
	writer.Flush()
	return nil
}

func ReadProofFile(path string, num, len int) ([][]byte, error) {
	if num <= 0 {
		err := errors.New("illegal label number")
		return nil, errors.Wrap(err, "read proof file error")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "read proof file error")
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	data := make([][]byte, num)
	for i := 0; i < num; i++ {
		label := make([]byte, len)
		n, err := reader.Read(label)
		if err != nil || n != len {
			err := errors.New(fmt.Sprint("read label error", err))
			return nil, errors.Wrap(err, "read proof file error")
		}
		data[i] = label

	}
	return data, nil
}

type Int64s []int64

func (i64s Int64s) Len() int {
	return len(i64s)
}

func (i64s Int64s) Size() int64 {
	return int64(len(i64s))
}

func (i64s Int64s) Less(i, j int) bool {
	return i64s[i] < i64s[j]
}

func (i64s Int64s) Swap(i, j int) {
	i64s[i], i64s[j] = i64s[j], i64s[i]
}

func RandString(lenNum int) string {
	str := strings.Builder{}
	length := 52
	for i := 0; i < lenNum; i++ {
		str.WriteString(CHARS[r.Intn(length)])
	}
	return str.String()
}
