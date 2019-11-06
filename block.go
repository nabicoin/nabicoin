package main

import (
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"strconv"
	"time"
)

// Block represents a block in the blockchain
type Block struct {
	Timestamp     int64
	Transactions  []*Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
	Height        int
	TypeSend      int
}

type tmpBlock struct {
	Transactions  []byte
	PrevBlockHash []byte
	Nonce         int
}

// NewBlock creates and returns Block
func NewBlock(transactions []*Transaction, prevBlockHash []byte, height int, typeSend int) *Block {
	block := &Block{time.Now().Unix(), transactions, prevBlockHash, []byte{}, 0, height, typeSend}
	tb := &tmpBlock{nil, prevBlockHash, 0}
	pow := NewProofOfWork(block, tb)
	pow.Run()

	return block
}

// NewGenesisBlock creates and returns genesis Block
func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{}, 0, 0)
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() []byte {
	var transactions [][]byte

	for _, tx := range b.Transactions {
		if len(tx.ID) != 0 {
			transactions = append(transactions, tx.Serialize())
		}
	}
	mTree := NewMerkleTree(transactions)

	return mTree.RootNode.Data
}

// Serialize serializes the block
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}

	return result.Bytes()
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		if err == io.EOF {

		} else {
			log.Panic(err)
		}
	}

	return &block
}

// Serialize serializes the block
func (b *tmpBlock) Serialize() []byte {
	//var result bytes.Buffer
	//varencoder := gob.NewEncoder(&result)

	//varerr := encoder.Encode(b)
	//varif err != nil {
	//var	log.Panic(err)
	//var}

	//varreturn result.Bytes()
	var data []byte
	data = append(data, b.Transactions...)
	data = append(data, b.PrevBlockHash...)
	data = append(data, strconv.Itoa(b.Nonce)...)

	return data[:]
}
