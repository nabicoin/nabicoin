package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"os"
	_ "runtime"
)

const dbFile = "blockchain.db"
const blocksBucket = "blocks"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

var prevblockHash []byte
var del_count int

// Blockchain implements interactions with a DB ^^*
type Blockchain struct {
	tip []byte
	db  *bolt.DB
}

// CreateBlockchain creates a new blockchain DB
func CreateBlockchain(address string) *Blockchain {
	dbFile := dbFile
	if dbExists(dbFile) {
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}

	var tip []byte

	cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
	genesis := NewGenesisBlock(cbtx)

	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			log.Panic(err)
		}

		err = b.Put(genesis.Hash, genesis.Serialize())
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("l"), genesis.Hash)
		if err != nil {
			log.Panic(err)
		}
		tip = genesis.Hash

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	bc := Blockchain{tip, db}

	return &bc
}

// NewBlockchain creates a new Blockchain with genesis Block
func NewBlockchain() *Blockchain {
	dbFile := dbFile
	if dbExists(dbFile) == false {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}

	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		tip = b.Get([]byte("l"))

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	bc := Blockchain{tip, db}

	return &bc
}

// AddBlock saves the block into the blockchain
func (bc *Blockchain) AddBlock(addr string, block *Block) {
	var existBlock *Block
	var sender_balance [20]int
	var receiver_balance [20]int
	IsExist := false
	addBlock := false

	if bytes.Equal(block.Hash, prevblockHash) {
		return
	}

	prevblockHash = block.Hash

	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex()

	err := bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		blockInDb := b.Get(block.Hash)

		if blockInDb != nil {
			IsExist = true
			existBlock = DeserializeBlock(blockInDb)
		}

		lastHash := b.Get([]byte("l"))
		lastBlockData := b.Get(lastHash)
		lastBlock := DeserializeBlock(lastBlockData)
	

		if block.Height == lastBlock.Height {
			if lastBlock.Height > 1 {
				/*if lastBlock.Timestamp > block.Timestamp {
					fmt.Println("[add block] delete last block")

					err := b.Delete([]byte("l"))
					if err != nil {
						log.Panic(err)
					}

					err = b.Put([]byte("l"), lastBlock.PrevBlockHash)
					if err != nil {
						log.Panic(err)
					}

					delete(blocksInTransit, lastBlock.Height)

					for _, tx := range block.Transactions {
						if txIDPool[hex.EncodeToString(tx.ID)] == hex.EncodeToString(tx.ID) {
							delete(txIDPool, hex.EncodeToString(tx.ID))
						}
					}

					sendGetData(addr, "block", nil, block.Height-1)

					//fmt.Printf("[add block] delete block %x\n", lastBlock.Hash)

					return nil
				}*/

			}
		}

		if block.Height == lastBlock.Height+1 {
			//check tx
			if len(block.Transactions) > 0 {
				//store receiver balance
				count := 0

				//check sender balance
				for _, tx := range block.Transactions {
					if len(tx.Vout) == 1 {
						count++

						continue
					}

					if tx.Vout[0].Value < 1 {
						count++

						continue
					}

					if tx.Vout[1].Value < 1 {
						count++

						continue
					}

					sum := tx.Vout[0].Value + tx.Vout[1].Value

					PubKeyHash := tx.Vout[1].PubKeyHash

					balance := 0

					UTXOs := UTXOSet.FindUTXO(PubKeyHash)

					for _, out := range UTXOs {
						if out.IsLockedWithKey(PubKeyHash) {
							balance += out.Value

							if balance == sum {
								break
							}
						}
					}

					if balance != sum {
						//log.Printf("sum: %d + %d =  %d, balance: %d\n",tx.Vout[0].Value, tx.Vout[1].Value, sum, balance)

						//sendVersion(remoteAddress, bc)
						if block.Height != 1 {
							if bytes.Equal(block.PrevBlockHash, lastHash) {
								log.Println("balance check fail!!!")
								//deleteTime = true
								return nil
							}
						}		
					}

					PubKeyHash = tx.Vout[0].PubKeyHash

					receiver_balance[count] = 0

					UTXOs = UTXOSet.FindUTXO(PubKeyHash)

					for _, out := range UTXOs {
						if out.IsLockedWithKey(PubKeyHash) {
							receiver_balance[count] += out.Value
						}
					}

					sender_balance[count] = tx.Vout[1].Value
					receiver_balance[count] += tx.Vout[0].Value
				}
			}

			if bytes.Equal(block.PrevBlockHash, lastHash) {
				del_count = 0

				var tb tmpBlock
				pow := NewProofOfWork(block, &tb)
				if pow.Validate() == false {
					fmt.Println("check hash: FALSE")

					return nil
				} else {
					//fmt.Println("check hash: PASS")
				}

				blockData := block.Serialize()

				err := b.Put(block.Hash, blockData)
				if err != nil {
					log.Panic(err)
				}

				err = b.Put([]byte("l"), block.Hash)
				if err != nil {
					log.Panic(err)
				}
				bc.tip = block.Hash

				for _, tx := range block.Transactions {
					if txIDPool[hex.EncodeToString(tx.ID)] != hex.EncodeToString(tx.ID) {
						txIDPool[hex.EncodeToString(tx.ID)] = hex.EncodeToString(tx.ID)
					}
				}

				newBlockData := b.Get(block.Hash)
				newBlock := DeserializeBlock(newBlockData)

				blocksInTransit[block.Height] = *newBlock

				if len(blocksInTransit) > maxBlocksOnMemory {
					for index, _ := range blocksInTransit {
						delete(blocksInTransit, index)

						break
					}
				}

				//runtime.Gosched()


				if addr != "127.0.0.1" {
					sendGetData(addr, "block", nil, block.Height+1)
				}

				//fmt.Printf("[add block] (height : %d) hash %x\n", block.Height, block.Hash)
				addBlock = true

				return nil
			} else if (lastBlock.Height > 1) {
				fmt.Println("[add block] delete last block")

				err := b.Delete([]byte("l"))
				if err != nil {
					log.Panic(err)
				}

				err = b.Put([]byte("l"), lastBlock.PrevBlockHash)
				if err != nil {
					log.Panic(err)
				}

				delete(blocksInTransit, lastBlock.Height)

				for _, tx := range block.Transactions {
					if txIDPool[hex.EncodeToString(tx.ID)] == hex.EncodeToString(tx.ID) {
						delete(txIDPool, hex.EncodeToString(tx.ID))
					}
				}

				sendGetData(addr, "block", nil, block.Height-1)

				//fmt.Printf("[add block] delete block %x\n", lastBlock.Hash)

				return nil
			}
		}

		sendGetData(addr, "block", nil, block.Height+1)

		return nil

	})
	if err != nil {
		log.Panic(err)
	}

	UTXOSet.Reindex()

	if addBlock == true {
		if len(block.Transactions) > 0 {
			count := 0

			//check balance
			for _, tx := range block.Transactions {
				if len(tx.Vout) == 1 {
					count++

					continue
				}

				if tx.Vout[0].Value < 1 {
					count++

					continue
				}

				if tx.Vout[1].Value < 1 {
					count++

					continue
				}

				PubKeyHash := tx.Vout[1].PubKeyHash
				s_balance := 0

				UTXOs := UTXOSet.FindUTXO(PubKeyHash)

				for _, out := range UTXOs {
					if out.IsLockedWithKey(PubKeyHash) {
						s_balance += out.Value
					}
				}

				PubKeyHash = tx.Vout[0].PubKeyHash
				r_balance := 0

				UTXOs = UTXOSet.FindUTXO(PubKeyHash)

				for _, out := range UTXOs {
					if out.IsLockedWithKey(PubKeyHash) {
						r_balance += out.Value
					}
				}

				if s_balance != sender_balance[count] || r_balance != receiver_balance[count] {
					//fmt.Printf("s_balance = %d\n", s_balance)
					//fmt.Printf("r_balance = %d\n", r_balance)
					//fmt.Printf("sender_balance = %d\n", sender_balance[count])
					//fmt.Printf("receiver_balance = %d\n", receiver_balance[count])

					err := bc.db.Update(func(tx *bolt.Tx) error {
						b := tx.Bucket([]byte(blocksBucket))

						lastHash := b.Get([]byte("l"))
						lastBlockData := b.Get(lastHash)
						lastBlock := DeserializeBlock(lastBlockData)

						err := b.Delete([]byte("l"))
						if err != nil {
							log.Panic(err)
						}

						err = b.Put([]byte("l"), lastBlock.PrevBlockHash)
						if err != nil {
							log.Panic(err)
						}
						
						delete(blocksInTransit, lastBlock.Height)

						return nil
					})
					if err != nil {
						log.Panic(err)
					}


					for _, tx := range block.Transactions {
						if txIDPool[hex.EncodeToString(tx.ID)] == hex.EncodeToString(tx.ID) {
							delete(txIDPool, hex.EncodeToString(tx.ID))
						}
					}

					sendGetData(addr, "block", nil, block.Height)

					return
				}

				count++
			}
		}

		fmt.Printf("[add block] (height : %d)\n", block.Height)
	}

	//sendVersion(addr, bc)
}

// FindTransaction finds a transaction by its ID
func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction is not found")
}

// FindUTXO finds all unspent transaction outputs and returns transactions with spent outputs removed
func (bc *Blockchain) FindUTXO() map[string]TXOutputs {
	UTXO := make(map[string]TXOutputs)
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

			Outputs:
			for outIdx, out := range tx.Vout {
				// Was the output spent?
				if spentTXOs[txID] != nil {
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx {
							continue Outputs
						}
					}
				}

				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			if tx.IsCoinbase() == false {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return UTXO
}

//whl add
func (bc *Blockchain) FindSpentTXO() map[string][]int {
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			if tx.IsCoinbase() == false {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return spentTXOs
}

// Iterator returns a BlockchainIterat
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tip, bc.db}

	return bci
}

// GetBestHeight returns the height of the latest block
func (bc *Blockchain) GetBestHeight() int {
	var lastBlock Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash := b.Get([]byte("l"))
		blockData := b.Get(lastHash)
		lastBlock = *DeserializeBlock(blockData)

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return lastBlock.Height
}

func (bc *Blockchain) GetBlockByHeight(blockHeight int) (Block, bool) {
	var block *Block

	if blockHeight != 0 {
		bci := bc.Iterator()

		for {
			block = bci.Next()
			if blockHeight == block.Height {
				//fmt.Println("[get block by height] FIND BLOCK!!!!!")

				return *block, true
			}

			if len(block.PrevBlockHash) == 0 {
				//fmt.Println("[get block by height] connot find block!!!!!")

				return *block, false
			}
		}
	}
	//fmt.Println("[get block by height] connot find block!!!!!")

	return *block, false
}

// GetBlock finds a block by its hash and returns it
func (bc *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		blockData := b.Get(blockHash)

		if blockData == nil {
			return errors.New("[GetBlock] Block is not found.")
		}

		block = *DeserializeBlock(blockData)

		return nil
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

// GetBlockHashes returns a list of hashes of all the blocks in the chain
func (bc *Blockchain) GetBlockHashes(Height int) [][]byte {
	var blocks [][]byte
	var tmpBlocks [][]byte
	bci := bc.Iterator()

	for {
		block := bci.Next()

		blocks = append(blocks, block.Hash)
		tmpBlocks = append(tmpBlocks, block.Hash)

		if len(block.PrevBlockHash) == 0 {
			break
		}

		if block.Height == Height {
			break
		}
	}

	//reverse
	for idx, _ := range tmpBlocks {
		if idx <= (len(blocks) / 2) {
			blocks[idx], blocks[(len(tmpBlocks)-1)-idx] = blocks[(len(tmpBlocks)-1)-idx], blocks[idx]
		} else {
			break
		}
	}

	return blocks
}

// MineBlock mines a new block with the provided transactions
func (bc *Blockchain) MineBlock(transactions []*Transaction, typeSend int) (*Block, error) {
	var lastHash []byte
	var lastHeight int

	for _, tx := range transactions {
		// TODO: ignore transaction if it's not valid
		if bc.VerifyTransaction(tx) != true {
			var emptyBlock Block
			return &emptyBlock, errors.New("ERROR: Invalid transaction")
		}
	}

	//get lastHeight, lastHash
	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))

		blockData := b.Get(lastHash)
		block := DeserializeBlock(blockData)

		lastHeight = block.Height

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	newLastHeight := lastHeight + 1

	//mining
	newBlock := NewBlock(transactions, lastHash, newLastHeight, typeSend)

	return newBlock, nil
}

// SignTransaction signs inputs of a Transaction
func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			//fmt.Print(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

// VerifyTransaction verifies transaction input signatures
func (bc *Blockchain) VerifyTransaction(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			fmt.Print(err)

			return false
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}

func dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}
