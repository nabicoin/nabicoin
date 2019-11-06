package main

import (
	"fmt"
	"log"
)

func (cli *CLI) sendDry(from, to string, amount int) {
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
