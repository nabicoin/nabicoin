package main

import (
	"fmt"
	_ "strconv"
)

func (cli *CLI) printChain(address string, height int) {
	if !ValidateAddress(address) {
		return
	}

	bc := NewBlockchain()

	bci := bc.Iterator()

	for {
		block := bci.Next()
		if block.Height == 0 {
			break
		}

		if block.Height == height {
			for _, tx := range block.Transactions {
				if len(tx.Vin[0].Txid) > 0 {
					senderAddress := GetAddress(tx.Vout[1].PubKeyHash)
					receiverAddress := GetAddress(tx.Vout[0].PubKeyHash)
					if string(senderAddress) == address || string(receiverAddress) == address {
						fmt.Printf("%v\n", tx.GetTxInfo())
					}
				}
			}
			break
		}
	}
}
