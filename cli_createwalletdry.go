package main

import "fmt"

func (cli *CLI) createWalletDry() {
	wallets, _ := NewWallets()
	address, public, private := wallets.CreateWallet()
	wallets.SaveToFile()

	fmt.Printf("address    : %s\n", address)
	fmt.Printf("public key : %s\n", public )
	fmt.Printf("private key: %s\n", private)
}
