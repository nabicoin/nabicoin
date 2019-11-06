package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"time"
	"log"
)

var (
	maxNonce = math.MaxInt64
)

const targetBits = 10

// ProofOfWork represents a proof-of-work
type ProofOfWork struct {
	block    *Block
	tmpblock *tmpBlock
	target   *big.Int
}

// NewProofOfWork builds and returns a ProofOfWork
func NewProofOfWork(b *Block, tb *tmpBlock) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	tb.Transactions = b.HashTransactions()
	tb.PrevBlockHash = b.PrevBlockHash
	tb.Nonce = b.Nonce

	pow := &ProofOfWork{b, tb, target}

	return pow
}

func (pow *ProofOfWork) getHash() []byte {
	data := pow.tmpblock.Serialize()
	hash := sha256.Sum256(data)

	return hash[:]
}

// Run performs a proof-of-work!
func (pow *ProofOfWork) Run() {
	defer ElapsedTime("check minning time", "start")()
	var hashInt big.Int
	nonce := 0
	pow.block.Nonce = nonce
	pow.tmpblock.Nonce = nonce

	for nonce < maxNonce {
		hash := pow.getHash()
		if math.Remainder(float64(nonce), 100000) == 0 {
			fmt.Printf("%x\n", hash)
		}
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {
			pow.block.Hash = hash[:]
			pow.block.Timestamp = time.Now().UTC().Unix()

			break
		} else {
			nonce++
			pow.block.Nonce = nonce
			pow.tmpblock.Nonce = nonce
		}
	}
}

// Validate validates block's PoW
func (pow *ProofOfWork) Validate() bool {
	isValid := bytes.Equal(pow.getHash(), pow.block.Hash)

	return isValid
}

func ElapsedTime(tag string, msg string) func() {
    if msg != "" {
        log.Printf("[%s] %s", tag, msg)
    }

    start := time.Now()
    return func() { log.Printf("[%s] Elipsed Time: %s", tag, time.Since(start)) }
}
