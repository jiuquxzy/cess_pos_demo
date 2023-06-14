# Proof of Idle Space

## Verifier call guide

### Init Verifier
Before using the verifier, it needs to be initialized first. 
Since the lightweight MHT is used in the verification process, the MHT object pool also needs to be initialized.
```go
import (
    "cess_pos_demo/pois"
	"cess_pos_demo/tree"
    "cess_pos_demo/expanders"
)

// key is cess_pos_demo.acc.RsaKey,be created by tee worker first.
// k,n,d respectively represent the number of layers of expanders, the number of nodes in each layer and the in-degree of each node.
// k,n,d are always set to 7, 1024*1024 and 64.
verifier:=pois.NewVerifier(key, k,n,d)

// init mht object pool to reduce memory allocation
// n is the number of nodes in each layer,HashSize indicates the size of each element,the default is 64 (bytes) Generally. 
tree.InitMhtPool(n,expanders.HashSize)
```

### Register Storage Miner

The verifier needs to keep information about every storage miner it interacts with.
So before using Proof of Space, you need to register miners.
```go
//minerID is storage miner's accountId,it is a byte slice
verifier.RegisterProverNode(minerID)
```

### PoIS setp 1:Receive Commits

first, receive idle file commits from a storage miner.
```go
//commits is a commits set of pois.CommitProof slice structure form miner
//commits can be transmitted using any communication protocol, and are generally serialized into a json byte sequence.
//Therefore, the received data needs to be decoded first and then handed over to the verifier for storage.
err:=verifier.ReceiveCommits(minerID,commits)
if err!=nil{
    //error handling code...
}

//if everythings is be ok,you need to response ok to miner.
// ...
```

### POIS setp 2:Generate Commit Challenges

After receiving the commits, it is necessary to generate commitchallenges to the storage miner, 
this step is to prove that the idle file commit by the miner is valid.
```go
//left and right is the bounds of commits you want to challenge,such as if you receive 16 idle file commits from miner,
//you can challenge these commits by set left=0,right=16.If you receive many commits,but just want to challenge a part,
//you can set left=0,right=num(number you want,num<= lenght of commits),and then left=num,right=others... set them in order.
chals, err := verifier.CommitChallenges(minerID, left, right)
if err!=nil{
    //error handling code...
}
// send chals to minner,chals is a slice of int64 slice,like [][]int64
// chals[i] represents a idle file commit challenge,including chals[i][0]=FileIndex,chals[i][1]=NodeIndex(last layer),
// chals[i][j]=(node(j-1)'s parent node)
```

### POIS setp 3:Verify Commit Proofs

The verifier needs to verify the commit challenges proof submitted by the storage miner.
```go
//commitProof, err := prover.ProveCommit(chals)

// chals is commit challenges generated by verifier before,commitProof is chals proof generated by miner.
// verifier need to compare chals's file and node index and commitProofs' in VerifyCommitProofs method.
err:=verifier.VerifyCommitProofs(minerID, chals, commitProof)
if err!=nil{
    //verification failed
    ////error handling code...
}
//verification success
//send ok to miner
```

### POIS step 4:Verify Acc Proof

If the commit proof verification is successful, the storage miner will submit the proof of acc.
```go
//accproof, err := prover.ProveAcc(indexsOfChallengedFile)

//chals is commit challenges generated by verifier before,accproof is generated by miner.
err = verifier.VerifyAcc(minerID, chals, accproof)
if err != nil {
	//verification failed
    ////error handling code...
}
//verification success,commit verify be done,verifier will update miner Info in VerifyAcc method.
//send ok to miner
```

### POIS step 5:Generate Space Proof

This work will be performed by CESS Node, which is compatible with proof of storage.
But when testing you can mock the execution.
```go
//num is the number of space challenges you want,it cannot exceed Count(idle file number for which commit verification has been passed).
spaceChals, err := verifier.SpaceChallenges(minerID, num)
if err!=nil{
    //error handling code...
}
//send spaceChals to miner,spaceChals same as commit chals,but one idle file just one node in last layer be challenged
```

### POIS step 6:Verify Space Proof

Verify the proof of space challenges submitted by the Storage Miner
```go
// spaceProof, err := prover.ProveSpace(spaceChals)

//spaceProof generated by miner
err = verifier.VerifySpace(minerID, spaceChals, spaceProof)
if err!=nil{
     //error handling code...
}
//send ok to miner
// if err==nil,verify space success,the tee worker needs to report the result to the chain, 
//which can be implemented directly in rust later.
```

### POIS step 7:Verify Deletion Proof

If the storage miner does not have enough space to store user files, it needs to delete some idle files, 
and the verifier needs to verify that the new accumulator is obtained by deleting the specified file from the previous accumulator.
```go
//chProof, Err := prover.ProveDeletion(number)
//chProof and Err are go channel,because deletion is a delayed operation that first sends a delete signal and then waits for other
//file-altering operations (such as generating or attesting files) to complete before starting the delete.
//Note that when testing the deletion proof, the code block that judges the remaining logical space needs to be commented out, 
//because if there is enough unproven space, the deletion proof is not required.
/*
code:
------------------------------------------------------
    // size := int64(expanders.HashSize) * num
    // if size < AvailableSpace {
    // 	ch <- nil
    // 	Err <- nil
    // 	return
    // }
------------------------------------------------------
space module:
------------------------------------------------------
|physical space                                      |
| ----------------------------------------------------
|logical space(defined by pois.AvailableSpace)       |
|-----------------------------------------------------
|user files   |idle files  |unproven or unused space |
------------------------------------------------------
*/

//delProof read from chProof,
err = verifier.VerifyDeletion(minerID, delProof)
if err!=nil{
    //error handling code...
}
//send ok to miner
```
