package test

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/util"
	"testing"
	"time"
)

func TestACC(t *testing.T) {
	data := make([][]byte, 1024)
	for i := 0; i < len(data); i++ {
		data[i] = []byte(util.RandString(256))
	}
	key := acc.RsaKeygen(2048)
	ts := time.Now()
	acc.GenerateWitness(key.G, key.N, data)
	t.Log("test acc success", time.Since(ts))
}
