package main

import "fmt"

func (cli *CLI) createWallet() {
	wallets, _ := NewWallets()
	address, _, _ := wallets.CreateWallet()
	wallets.SaveToFile()

	fmt.Printf("Your new address: %s\n", address)
}
