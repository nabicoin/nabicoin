package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
)

func (cli *CLI) getBalance(address string) {
	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}

	data := GetBalance{address}
	payload := gobEncode(data)
	request := append(commandToBytes("getBalance"), payload...)

	localhostAddr := getLocalhost()
	localhostAddr += ":" + port

	sendData(localhostAddr, request)

	nodeAddress := fmt.Sprintf("127.0.0.1:%s", port)

	ln, err := net.Listen(protocol, nodeAddress)
	if err != nil {
		log.Panic(err)
	}

	defer func() {
		ln.Close()
	}()

	conn, err := ln.Accept()
	if err != nil {
		log.Panic(err)
	}

	defer func() {
	}()

	balanceText, err_read := ioutil.ReadAll(conn)
	if err_read != nil {
		//log.Panic(err)
		fmt.Println(err)
		conn.Close()

		return
	}

	conn.Close()

	balance := bytesToString(balanceText[:])
	log.Println(balance)
}
