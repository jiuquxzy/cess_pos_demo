package pois

import (
	"cess_pos_demo/acc"
	"cess_pos_demo/expanders"
	"cess_pos_demo/tree"
	"log"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

/*
	prove Service provides a series of apis for proof of idle space,
	which are wrappers for the proof and verification modules,
	and demonstrates the best practices and how we expect it to be used.


*/

const (
	DEFAULT_NODES   = 1024 * 1024
	DEFAULT_LAYERS  = 7  //[0,7],so it is 8 layers
	DEFAULT_DEGREE  = 64 //actually there are 64+1 degrees
	DEFAULT_TIMEOUT = 30 * time.Second
	INTERVAL        = 10 * time.Second
	DEFAULT_BUFSIZE = 4
)

type CommitResult struct {
	Ok  bool
	Num int64
}

// ConfigProver set base configuration(available space,max commit proof thread,idle file size) for prover
func ConfigProver(space int64, thread int, size int64) {
	AvailableSpace = space
	MaxCommitProofThread = thread
	FileSize = size
}

// InitProveService perform the necessary initialization work,
// including init prover, mht and run idle file generation server.
func InitProveService(k, n, d int64, id []byte, fPath, accPath string, key acc.RsaKey) error {
	//init prover
	err := InitProver(
		k, n, d,
		id, fPath, accPath, key,
	)
	if err != nil {
		return errors.Wrap(err, "init pois service error")
	}

	//init light weight merkel hash tree pool
	tree.InitMhtPool(int(n), expanders.HashSize)

	//run idle file generation server
	GetProver().RunIdleFileGenerationServer(MaxCommitProofThread)
	return nil
}

// AutoGenerateIdleFileService auto generate idle file if there is enough space
func AutoGenerateIdleFileService(closed chan struct{}) {
	prover := GetProver()
	if prover == nil {
		return
	}
	go func() {
		num := int64(MaxCommitProofThread)
		size := FileSize * num * (prover.Expanders.K + 1)
		for {
			select {
			case <-closed:
				return
			default:
				prover.GenerateFile(num)
			}
			if AvailableSpace < size {
				num = 1
			} else {
				num = int64(MaxCommitProofThread)
			}
		}
	}()
}

func CommitIdleFileService(bufSize int) (
	<-chan []Commit, //commits channel
	func(CommitResult), //func of set commit result
	func(), //func close the commit idle file service
) {
	var rollback atomic.Bool
	prover := GetProver()
	if prover == nil {
		return nil, nil, nil
	}
	if bufSize <= 0 {
		bufSize = DEFAULT_BUFSIZE
	}
	commitCh, result := make(chan []Commit, bufSize), make(chan CommitResult, bufSize)
	closed := make(chan struct{}, 1)
	//generate commit service
	go func() {
		ticker := time.NewTicker(INTERVAL)
		var num, count int64
		for {
			select {
			case <-closed:
				return
			case <-ticker.C:
			}
			if rollback.Load() {
				continue
			}
			count++
			num = prover.Generated.Load() - prover.Commited.Load()
			if num < int64(MaxCommitProofThread) || count < 36 {
				continue
			}
			commits, err := prover.GetCommits(num)
			if err != nil {
				log.Println("get commits error", err)
			}
			commitCh <- commits
			count = 0
		}
	}()

	//receive commit result service
	go func() {
		timeout := time.NewTicker(DEFAULT_TIMEOUT)
		var res CommitResult
		for {
			select {
			case res = <-result:
			case <-timeout.C:
				rollback.Store(false)
			}
			if !res.Ok {
				rollback.Store(true)
			}
			if rollback.Load() {
				prover.CommitRollback(res.Num)
			}
			timeout.Reset(DEFAULT_TIMEOUT)
		}
	}()
	return commitCh, func(cr CommitResult) { result <- cr }, func() { close(closed) }
}

//func ProveCommitAndAccService(chalCh <-chan [][]int64,commitProofCh,result <-chan CommitResult, closed chan struct{})(chan [][]CommitProof,)
// func ProveCommitAndAccService(bufSize int)(
// 	<-chan [][]CommitProof,
// 	func ([][]int64),
// 	func (CommitResult),

// ){

// }
