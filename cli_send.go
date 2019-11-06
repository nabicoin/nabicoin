package main

import ()

func (cli *CLI) send(from, to string, amount int) {
	data := SendCoin{from, to, amount}
	payload := gobEncode(data)
	request := append(commandToBytes("sendCoin"), payload...)

	localhostAddr := getLocalhost()
	localhostAddr += ":" + port

	sendData(localhostAddr, request)
}
