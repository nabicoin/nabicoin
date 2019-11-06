package main

import (
	"fmt"
	"log"
	"strings"
)

func createWalletDry() (string, string, string) {
	wallets, _ := NewWallets()
	address, public, private := wallets.CreateWallet()

	return address, public, private
}

func sendDry(from, to string, amount int) {
	if !ValidateAddress(from) {
		log.Panic("ERROR: Sender address is not valid")
	}
	if !ValidateAddress(to) {
		log.Panic("ERROR: Recipient address is not valid")
	}
	if amount < 1 {
		log.Panic("ERROR: amount is not valid")
	}

	bc := NewBlockchain()
	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex()

	defer bc.db.Close()

	wallets, err := NewWallets()
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)

	tx, err := NewUTXOTransaction(&wallet, to, amount, &UTXOSet)
	if err != nil {
		log.Panic("ERROR: make tx is fail.")
	}

	if bc.VerifyTransaction(tx) == false {
		log.Panic("ERROR: #tx is fault.")
	} else {
		fmt.Println("[SUCCESS] tx is ok.")
	}

	if len(knownNodes) == 0 {
		log.Panic("ERROR: no have node address")
	}

	for idx := 0; idx < len(knownNodes); idx++ {
		sendTx(knownNodes[idx], tx)
		fmt.Println("[send Tx]")
	}
}

func  txInfo(address string, blockno int) string {
	var info []string

	if !ValidateAddress(address) {
		return "address is woring"
	}

	bc := NewBlockchain()

	bci := bc.Iterator()

	for {
		block := bci.Next()
		if block.Height == 0 {
			break
		}

		if block.Height == blockno {
			for _, tx := range block.Transactions {
				if len(tx.Vin[0].Txid) > 0 {
					senderAddress := GetAddress(tx.Vout[1].PubKeyHash)
					receiverAddress := GetAddress(tx.Vout[0].PubKeyHash)
					if string(senderAddress) == address || string(receiverAddress) == address {
						info = append(info, tx.GetTxInfo())
						fmt.Printf("%v\n", tx.GetTxInfo())
					}
				}
			}
			break
		}
	}

	infos := strings.Join(info, " ")

	return infos
}
