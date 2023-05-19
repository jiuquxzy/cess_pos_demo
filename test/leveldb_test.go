package test

import (
	"cess_pos_demo/util"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

func TestLevelDB(t *testing.T) {
	path := "graph/stacked_expanders/g001"
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		t.Fatal("open level db error", err)
	}
	st := time.Now()
	for i := int64(0); i < 1024*1024*4; i++ {
		db.Get(util.Int64Bytes(i), nil)
	}
	t.Log("time:", time.Since(st))
}
