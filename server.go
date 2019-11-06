package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/leekchan/accounting"
	"github.com/sparrc/go-ping"

	"net/http"

	"github.com/labstack/echo"
)

const (
	sendCoinMine = iota
	emptyMine
	nomalMine
)

type Balance struct {
	Address string `json:"address"`
	Balance int    `json:"balance"`
}

type Createwallet struct {
	Address string `json:"address"`
	Public  string `json:"public"`
	Private string `json:"private"`
}

type GetTx struct {
	Address string   `json:"address"`
	TxInfos []string `json:"txinfos"`
}

const protocol = "tcp"
const nodeVersion = 1
const commandLength = 122

const mainServerIP = "xxx.xxx.xxx.xxx"
const port = "60199"
const mainServer = mainServerIP + ":" + port
const key = "xxxxxxxxxxxxxxxx"
const maxBlocksOnMemory = 2000000 //2G?

var flagMining = false
var waitSecond int
var remoteAddress string
var nodeAddress string
var miningAddress string
var startMiningTime time.Time
var startVersionTime time.Time
var startAddrTime time.Time
var startCleanTime time.Time
var startAddingBlockTime time.Time
var knownNodes = []string{mainServer}
var nodes = make(map[string]string)
var mempool = make(map[string]Transaction)

//var storemempool = make(map[string]Transaction)
var txIDPool = make(map[string]string)
var blocksInTransit = make(map[int]Block)
var mutex = new(sync.Mutex)
var wg sync.WaitGroup
var blockWG sync.WaitGroup
var cryptoBlock, _ = aes.NewCipher([]byte(key))

type addr struct {
	AddrList []string
}

type block struct {
	AddrFrom string
	Block    []byte
}

type getdata struct {
	AddrFrom string
	Type     string
	ID       []byte //none use
	Height   int
}

type inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type tx struct {
	AddrFrom    string
	Transaction []byte
}

type versionInfo struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

type nodePing struct {
	node     string
	pingTime time.Duration
}

type nodePings []nodePing

type ByPingTime struct {
	nodePings
}

type GetBalance struct {
	Address string
}

type SendCoin struct {
	From   string
	To     string
	Amount int
}

func commandToBytes(command string) []byte {
	var bytes [commandLength]byte

	for i, c := range command {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

func bytesToCommand(bytes []byte) string {
	var command []byte

	for _, b := range bytes {
		if b != 0x0 {
			command = append(command, b)
		}
	}

	return fmt.Sprintf("%s", command)
}

func stringToBytes(str string) []byte {
	var bytes []byte

	for _, c := range str {
		bytes = append(bytes, byte(c))
	}

	return bytes[:]
}

func bytesToString(bytes []byte) string {
	var str []byte

	for _, b := range bytes {
		if b != 0x0 {
			str = append(str, b)
		}
	}

	return fmt.Sprintf("%s", str)
}

func extractCommand(request []byte) []byte {
	return request[:commandLength]
}

func sendAddr(address string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)

			knownNodes = nil

			knownNodes = append(knownNodes, mainServer)
		}
	}()

	if len(knownNodes) == 0 {
		return
	}

	payload := gobEncode(knownNodes)
	request := append(commandToBytes("addr"), payload...)

	sendData(address, request)
}

func sendBlock(addr string, b *Block) bool {
	data := block{nodeAddress, b.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes("block"), payload...)

	return sendData(addr, request)
}

func sendData(addr string, data []byte) bool {
	var timeout time.Duration

	timeout = 1 * time.Second
	dial := net.Dialer{Timeout: timeout}

	for i := 0; i < 10; i++ {
		//fmt.Printf("[send data] try send to %s\n", addr)

		conn, err := dial.Dial(protocol, addr)
		if err != nil {
			//fmt.Println("dial fail!")

			continue
		} else {
			defer conn.Close()

			_, err = io.Copy(conn, bytes.NewReader(data))
			if err != nil {
				log.Println(err)
			} else {
				//fmt.Printf("[send data] sending success!!!")
			}

			return true
		}
	}

	return false
}

func sendGetData(address, kind string, id []byte, height int) bool {
	payload := gobEncode(getdata{nodeAddress, kind, id, height})
	request := append(commandToBytes("getdata"), payload...)

	return sendData(address, request)
}

func sendTx(addr string, tnx *Transaction) bool {
	data := tx{nodeAddress, tnx.Serialize_nomal()}
	payload := gobEncode(data)
	request := append(commandToBytes("tx"), payload...)

	return sendData(addr, request)
}

func sendVersion(addr string, bc *Blockchain) bool {
	bestHeight := bc.GetBestHeight()
	payload := gobEncode(versionInfo{nodeVersion, bestHeight, nodeAddress})

	request := append(commandToBytes("version"), payload...)

	return sendData(addr, request)
}

func handleGetBalance(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload GetBalance

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Println(err)

		return
	}

	address := payload.Address

	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}

	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex() //이거 없으면 잔돈이 잘못 표시됨

	balance := 0
	pubKeyHash := Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

	UTXOs := UTXOSet.FindUTXO(pubKeyHash)

	for _, out := range UTXOs {
		if out.IsLockedWithKey(pubKeyHash) {
			balance += out.Value
		}
	}

	addr := fmt.Sprintf("127.0.0.1:%s", port)

	ac := accounting.Accounting{Symbol: "", Precision: 0}
	//fmt.Printf("Balance of '%s': %s\n", address, ac.FormatMoney(balance))

	balanceValue := fmt.Sprintf("Balance of '%s': %s\n", address, ac.FormatMoney(balance))

	balanceText := stringToBytes(balanceValue)

	sendData(addr, balanceText)
}

func handleSendCoin(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload SendCoin

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Println(err)

		return
	}

	if len(knownNodes) == 0 {
		return
	}

	from := payload.From
	to := payload.To
	amount := payload.Amount

	if from == miningAddress {
		log.Println("Not allow mining address")

		return
	}

	if !ValidateAddress(from) {
		log.Println("ERROR: Sender address is not valid")

		return
	}
	if !ValidateAddress(to) {
		log.Println("ERROR: Recipient address is not valid")

		return
	}

	mutex.Lock()

	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex()

	wallets, err := NewWallets()
	if err != nil {
		log.Panic(err)
	}

	localhostAddr := getLocalhost()

	nodeAddress = fmt.Sprintf("%s:%s", localhostAddr, port)

	readAddressFromFile()

	if len(knownNodes) > 1 {
		ping_address()
	}

	wallet := wallets.GetWallet(from)

	tx, errtx := NewUTXOTransaction(&wallet, to, amount, &UTXOSet)
	if errtx != nil {
		mutex.Unlock()

		return
	}

	if bc.VerifyTransaction(tx) == false {
		fmt.Println("#maked tx is fault.")

		mutex.Unlock()

		return
	} else {
		fmt.Println("#maked tx is ok.")
	}

	sendTx(nodeAddress, tx)

	for idx := 0; idx < len(knownNodes); idx++ {
		sendTx(knownNodes[idx], tx)
	}

	mutex.Unlock()
}

func handleAddr(request []byte, bc *Blockchain) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	var buff bytes.Buffer
	var payload []string

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Println(err)

		return
	}

	addrList := make(map[string]int)

	for idx := 0; idx < len(payload); idx++ {
		if len(payload[idx]) > 0 {
			addrList[payload[idx]] = 0
		}
	}

	for idx := 0; idx < len(knownNodes); idx++ {
		if len(knownNodes[idx]) > 0 {
			addrList[knownNodes[idx]] = 0
		}
	}

	if len(addrList) > 0 {
		knownNodes = nil
	}

	var addKnownNodes []string

	for key, _ := range addrList {
		if len(key) > 0 {
			addKnownNodes = append(addKnownNodes, key)
			//fmt.Printf("[handle addr] known node : %s(%d)\n", key, len(knownNodes))
		}
	}

	knownNodes = addKnownNodes[:]

	//fmt.Printf("[handle addr pre] There are %d known nodes now!\n", len(knownNodes))
	currentTime := time.Now()
	elapsedMiningTime := currentTime.Sub(startAddrTime)

	if elapsedMiningTime.Seconds() > (60 * 1) {
		startAddrTime = time.Now()
		ping_address()
	}

	//writeAddressToFile()
	//fmt.Printf("[handle addr after] There are %d known nodes now!\n", len(knownNodes))
}

func handleBlock(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload block

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	payload.AddrFrom = remoteAddress

	blockData := payload.Block
	block := DeserializeBlock(blockData)

	//localBestHeight := bc.GetBestHeight()
	bc.AddBlock(payload.AddrFrom, block)
}

func handleGetData(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload getdata

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	payload.AddrFrom = remoteAddress

	if payload.Type == "block" {
		height := payload.Height
		if height == 0 {
			return
		}

		//fmt.Printf("[handleGetData] GetBlockByHeight:%d\n", height)
		//block, isHave := bc.GetBlockByHeight(height)

		block, isHave := blocksInTransit[height]

		if isHave && (block.Height > 0) {
			if sendBlock(payload.AddrFrom, &block) {
				//fmt.Println("[send block] send ok")
			}
		} else {
			//fmt.Printf("[handleGetData] GetBlockByHeight:%d\n", height)
			block, isHave := bc.GetBlockByHeight(height)

			if isHave {
				if sendBlock(payload.AddrFrom, &block) {
					//fmt.Println("[send block] send ok")
				}
			}
		}

		//runtime.Gosched()
	}

	//	if payload.Type == "tx" {
	//		txID := hex.EncodeToString(payload.ID)
	//		tx := storemempool[txID]
	//
	//		sendTx(payload.AddrFrom, &tx)
	//	}

	if payload.Type == "addr" {
		sendAddr(payload.AddrFrom)
	}
}

func handleTx(request []byte, bc *Blockchain) {
	//fmt.Println("[handle tx] handleTx")
	var buff bytes.Buffer
	var payload tx

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	//fmt.Printf("[handle Tx] remoteAddress = %s\n", remoteAddress)
	payload.AddrFrom = remoteAddress

	txData := payload.Transaction
	tx := DeserializeTransaction(txData)

	if txIDPool[hex.EncodeToString(tx.ID)] == hex.EncodeToString(tx.ID) {
		//fmt.Println("[handle tx] not new tx")

		currentTime := time.Now()
		elapsedCleanTime := currentTime.Sub(startCleanTime)
		if elapsedCleanTime.Seconds() < (60 * 1) {
			return
		}

		for key, _ := range txIDPool {
			delete(txIDPool, key)
		}
	}

	//if bc.VerifyTransaction(&tx) == false {
	//	//fmt.Println("[handle tx] tx is not ok")

	//	sendVersion(payload.AddrFrom, bc)

	//	return

	//}

	if len(mempool) == 0 {
		startMiningTime = time.Now()
	}
	//mempool[hex.EncodeToString(tx.ID)] = tx
	mempool[string(tx.ID)] = tx

	//storemempool[hex.EncodeToString(tx.ID)] = tx

	txIDPool[hex.EncodeToString(tx.ID)] = hex.EncodeToString(tx.ID)
	startCleanTime = time.Now()

	//fmt.Printf("[handle]add tx.ID to tx id pool(%s)\n", hex.EncodeToString(tx.ID))
	//fmt.Printf("[handle tx] history tx count: %d\n", len(storemempool))

	//fmt.Printf("[tx] known node count : %d\n", len(knownNodes))
	for idx := 0; idx < len(knownNodes); idx++ {
		sendTx(knownNodes[idx], &tx)
		//sendData(knownNodes[idx], request)
	}

	fmt.Printf("[handle tx] mempool size = %d(%x)\n", len(mempool), tx.ID)

	if (len(mempool) >= 40) && (len(miningAddress) > 0) {
		//if (len(mempool) >= 1) && (len(miningAddress) > 0) { //for test
		defer wg.Done()
		wg.Add(1)

		startMiningTime = time.Now()

		var txs []*Transaction
		txs = nil

		for key, _ := range mempool {
			//if bc.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
			//} else {
			//	fmt.Println("[handle tx] Not Verify Trasaction!!!")
			//}

			delete(mempool, key)
		}

		if len(txs) == 0 {
			fmt.Println("[handle tx] All transactions are invalid! Waiting for new ones...")

			return
		}

		go func() {
			defer wg.Done()
			wg.Wait()

			cbTx := NewCoinbaseTX(miningAddress, "")
			txs = append(txs, cbTx)

			mutex.Lock()

			wg.Wait() //2019.06.16

			log.Println("[handle tx] ========== start mining ==========")

			wg.Add(1)

			time.Sleep(2)

			newBlock, err := bc.MineBlock(txs, nomalMine)

			if err != nil {
				log.Println(err)

				mutex.Unlock()

				return
			}

			bc.AddBlock(nodeAddress, newBlock)

			log.Println("[handle tx] =======!!!!New block is mined!!!!=======")

			for _, node := range knownNodes {
				if nodeAddress != node {
					//fmt.Printf("[handle tx] try send version to %s", node)

					sendVersion(node, bc)
				}
			}
			mutex.Unlock()
		}()
	}

}

func emptyMining(bc *Blockchain) {
	var txs []*Transaction

	startMiningTime = time.Now()
	defer wg.Done()

	txs = nil

	for key, tx := range mempool {
		//if bc.VerifyTransaction(&tx) {
		txs = append(txs, &tx)
		//} else {
		//	fmt.Println("[miningk] Not Verify Trasaction!!!")
		//}

		delete(mempool, key)
	}

	cbTx := NewCoinbaseTX(miningAddress, "")
	txs = append(txs, cbTx)

	mutex.Lock()

	wg.Wait()

	wg.Add(1)

	log.Println("[mining] ========== start mining ==========")

	newBlock, err := bc.MineBlock(txs, emptyMine)

	if err != nil {
		log.Printf("\n[mining] =======!!!!Fail!!!!=======")
		mutex.Unlock()

		return
	}

	bc.AddBlock(nodeAddress, newBlock)

	log.Println("[mining] =======!!!!New block is mined!!!!=======")

	for _, node := range knownNodes {
		if nodeAddress != node {
			//fmt.Printf("[handle tx] try send version to %s", node)

			sendVersion(node, bc)
		}
	}

	//if len(txIDPool) > 0 {
	//	for key, _ := range txIDPool {
	//		delete(txIDPool, key)
	//	}
	//}

	mutex.Unlock()
}

func handleVersion(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload versionInfo

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	if len(remoteAddress) == 0 {
		return
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	payload.AddrFrom = remoteAddress

	myBestHeight := bc.GetBestHeight()
	foreignerBestHeight := payload.BestHeight

	if myBestHeight < foreignerBestHeight {
		sendAddr(payload.AddrFrom)

		//fmt.Printf("[handle version] send get block with local highest height + 1:%d\n", myBestHeight+1)

		sendGetData(payload.AddrFrom, "block", nil, myBestHeight+1)
		sendVersion(payload.AddrFrom, bc)
	} else if myBestHeight > foreignerBestHeight {
		sendVersion(payload.AddrFrom, bc)

		//fmt.Printf("[handle version] send version to %s\n", payload.AddrFrom)
	} else {
		sendAddr(payload.AddrFrom)
		sendGetData(payload.AddrFrom, "addr", nil, 0)
		sendGetData(payload.AddrFrom, "block", nil, myBestHeight)
		sendVersion(payload.AddrFrom, bc)
	}

	if !nodeIsKnown(payload.AddrFrom) {
		sendAddr(payload.AddrFrom)
		knownNodes = append(knownNodes, payload.AddrFrom)
	}
}

func handleConnection(conn net.Conn, bc *Blockchain) {
	defer func() {
		conn.Close()
	}()

	tmpAddr := conn.RemoteAddr().String()
	strTmp := strings.Split(tmpAddr, ":")
	remoteAddress = strTmp[0] + ":" + port

	//fmt.Println(remoteAddress)

	request, err := ioutil.ReadAll(conn)
	if err != nil {
		//log.Panic(err)
		fmt.Println(err)

		return
	}
	command := bytesToCommand(request[:commandLength])

	switch command {
	case "addr":
		//fmt.Printf("[handle connection] Received %s command\n", command)
		handleAddr(request, bc)
	case "block":
		//fmt.Printf("[handle connection] Received %s command\n", command)

		wg.Wait()
		handleBlock(request, bc)
	case "getdata":
		//fmt.Printf("[handle connection] Received %s command\n", command)
		handleGetData(request, bc)
	case "tx":
		//fmt.Printf("[handle connection] Received %s command\n", command)
		handleTx(request, bc)
	case "version":
		//fmt.Printf("[handle connection] Received %s command\n", command)
		go handleVersion(request, bc)
	case "sendCoin":
		go func() {
			wg.Wait()

			handleSendCoin(request, bc)
		}()
		//fmt.Printf("[handle connection] Received %s command\n", command)
	case "getBalance":
		//fmt.Printf("[handle connection] Received %s command\n", command)
		go handleGetBalance(request, bc)
	default:
		//fmt.Println("[handle connection] Unknown command!")
	}
}

func getLocalhost() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		os.Stderr.WriteString("Oops: " + err.Error() + "\n")
		os.Exit(1)
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localhostAddr := ipnet.IP.String()

				//fmt.Println(localhostAddr + ":" + port)

				return localhostAddr
			}
		}
	}

	//fmt.Println("localhost")

	return "localhost"
}

func ping_address() {
	if len(knownNodes) > 1 {
		//testNodeConnectable()

		ping_sort_address()

		if len(knownNodes) == 0 {
			knownNodes = append(knownNodes, mainServer)
		}
	}
}

// StartServer starts a node
func StartServer(minerAddress string) {
	fmt.Println("       ||\\\\     ||      //\\\\        =========      ||")
	fmt.Println("       || \\\\    ||     //  \\\\      ||       ||     ||")
	fmt.Println("       ||  \\\\   ||    //    \\\\     ||==========    ||")
	fmt.Println("       ||   \\\\  ||   //======\\\\    ||         ||   ||")
	fmt.Println("       ||    \\\\ ||  //        \\\\   ||         ||   ||")
	fmt.Println("       ||     \\\\|| //          \\\\   ===========    || - coin ver 1.0.0")

	waitSecond := 300

	localhostAddr := getLocalhost()

	nodeAddress = fmt.Sprintf("%s:%s", localhostAddr, port)
	knownNodes = append(knownNodes, nodeAddress)

	miningAddress = minerAddress
	ln, err := net.Listen(protocol, nodeAddress)
	if err != nil {
		log.Panic(err)
	}
	defer func() {
		ln.Close()
		createAddressFile()
		writeAddressToFile()

		fmt.Println("EXIT NAVI-COIN SYSTEM")
	}()

	//log.Printf("Listen: %s\n", nodeAddress)

	startMiningTime = time.Now()
	startCleanTime = time.Now()
	startAddingBlockTime = time.Now()
	startAddrTime = time.Now()

	bc := NewBlockchain()

	//read, sort, delete duplication
	readAddressFromFile()

	testNodeConnectable()

	ping_address()

	bestHeight := bc.GetBestHeight()

	startHeight := 0

	if bestHeight >= maxBlocksOnMemory {
		startHeight = bestHeight - maxBlocksOnMemory
	}

	bci := bc.Iterator()

	for {
		block := bci.Next()

		blocksInTransit[block.Height] = *block
		fmt.Printf("loaded block height: %d\n", block.Height)

		if len(block.Transactions) > 0 {
			for _, tx := range block.Transactions {
				txIDPool[hex.EncodeToString(tx.ID)] = hex.EncodeToString(tx.ID)
			}
		}

		if block.Height <= startHeight {
			break
		}
	}

	log.Println("loading blocks is completed")

	for _, node := range knownNodes {
		if nodeAddress != node {
			//fmt.Printf("try send version to %s", node)

			sendVersion(node, bc)
		}
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		ln.Close()
		createAddressFile()
		writeAddressToFile()

		fmt.Println("EXIT NAVI-COIN SYSTEM")
		os.Exit(1)
	}()

	if len(miningAddress) > 0 {
		go func() {
			for {
				bestHeight = bc.GetBestHeight()

				currentTime := time.Now()
				elapsedMiningTime := currentTime.Sub(startMiningTime)

				if int(elapsedMiningTime.Seconds()) > waitSecond {
					s := rand.NewSource(time.Now().UnixNano())
					r := rand.New(s)
					waitSecond = r.Intn(540) + 60

					emptyMining(bc)
				} else if len(mempool) >= 1 {
					if elapsedMiningTime.Seconds() > 10 {
						emptyMining(bc)
					}
				} else if bestHeight < 0 {
					emptyMining(bc)
				}

				runtime.Gosched()
			}
		}()
	}

	rest := echo.New()

	rest.GET("/balance/:address", func(c echo.Context) error {
		address := c.Param("address")

		if !ValidateAddress(address) {
			return c.String(http.StatusOK, "fail!")
		}

		UTXOSet := UTXOSet{bc}
		UTXOSet.Reindex() //이거 없으면 잔돈이 잘못 표시됨

		balance := 0
		pubKeyHash := Base58Decode([]byte(address))
		pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

		UTXOs := UTXOSet.FindUTXO(pubKeyHash)

		for _, out := range UTXOs {
			balance += out.Value
		}

		u := &Balance{
			Address: address,
			Balance: balance,
		}

		return c.JSON(http.StatusOK, u)
	})

	rest.GET("/send/:from/:to/:amount", func(c echo.Context) error {
		from := c.Param("from")
		to := c.Param("to")
		amountStr := c.Param("amount")
		if len(from) == 0 {
			return c.String(http.StatusOK, "send fail!")
		}

		if len(to) == 0 {
			return c.String(http.StatusOK, "send fail!")
		}

		if len(amountStr) == 0 {
			return c.String(http.StatusOK, "send fail!")
		}

		amount, errAtoi := strconv.Atoi(amountStr)
		if errAtoi != nil {
			return c.String(http.StatusOK, "send fail!")
		}

		data := SendCoin{from, to, amount}
		payload := gobEncode(data)
		request := append(commandToBytes("sendCoin"), payload...)

		localhostAddr := getLocalhost()
		localhostAddr += ":" + port

		result := sendData(localhostAddr, request)

		if result == true {
			return c.String(http.StatusOK, "send success!")
		} else {
			return c.String(http.StatusOK, "send fail!")
		}
	})

	rest.GET("/createwallet", func(c echo.Context) error {
		wallets, _ := NewWallets()
		address, public, private := wallets.CreateWallet()
		wallets.SaveToFile()

		u := &Createwallet{
			Address: address,
			Public:  public,
			Private: private,
		}

		return c.JSON(http.StatusOK, u)
	})

	rest.GET("/txinfo/:address", func(c echo.Context) error {
		address := c.Param("address")
		if !ValidateAddress(address) {
			return c.String(http.StatusOK, "fail!")
		}

		bci := bc.Iterator()

		var txInfos []string
		for {
			block := bci.Next()
			if block.Height == 0 {
				break
			}

			for _, tx := range block.Transactions {
				if len(tx.Vin[0].Txid) > 0 {
					senderAddress := GetAddress(tx.Vout[1].PubKeyHash)
					receiverAddress := GetAddress(tx.Vout[0].PubKeyHash)
					if string(senderAddress) == address || string(receiverAddress) == address {
						txInfos = append(txInfos, tx.GetTxInfo())
					}
				}
			}
		}

		txinfo := &GetTx{
			Address: address,
			TxInfos: txInfos,
		}

		return c.JSON(http.StatusOK, txinfo)
	})

	go func() {
		rest.Logger.Fatal(rest.Start(":1331"))
	}()

	for {
		//fmt.Println("[server] wait.....")
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}

		//fmt.Println("[server] accepted...")
		handleConnection(conn, bc)
	}
}

func testNodeConnectable() {
	//fmt.Println("[test node] test connect to node and sort node")
	//fmt.Printf("input node address count: %d\n", len(knownNodes))

	if len(knownNodes) > 1 {
		var timeout time.Duration

		knownNodes = nil

		timeout = 1 * time.Second
		dial := net.Dialer{Timeout: timeout}

		for node, _ := range nodes {
			conn, err := dial.Dial(protocol, node)
			if err != nil {
				if node != mainServer {
					delete(nodes, node)

					fmt.Printf("%s node is deleted\n", node)

					continue
				}
			} else {
				conn.Close()
			}

			knownNodes = append(knownNodes, node)
		}

		knownNodes = append(knownNodes, mainServer)

		fmt.Printf("%d node addresses are waken\n", len(knownNodes))
	}
}

func createAddressFile() {
	file, err := os.OpenFile(".knownNodes", os.O_CREATE|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		fmt.Println(err)
	} else {
		file.Close()
	}
}

func readAddressFromFile() {
START_READ_FILE:
	nodeFromFile, err := readLines(".knownNodes")
	if err != nil {
		//fmt.Printf("readLine's err: %s", err)
		createAddressFile()

		goto START_READ_FILE
	}

	knownNodes = nil
	for _, node := range nodeFromFile {
		//fmt.Printf("readed addr : %s\n", node)
		address := make([]byte, 16)

		cryptoNode := []byte(node)

		if len(cryptoNode) < 16 {
			continue
		}

		cryptoNode = cryptoNode[:16]
		cryptoBlock.Decrypt(address, cryptoNode)

		posColon := strings.Index(string(address), ":")
		address = address[:posColon+1]
		strAddress := string(address)
		strAddress += port
		knownNodes = append(knownNodes, strAddress)
		nodes[strAddress] = strAddress
	}

	if len(knownNodes) < 1 {
		knownNodes = append(knownNodes, mainServer)
		nodes[mainServer] = mainServer
	}
}

func writeAddressToFile() {
	write_err := writeLines(knownNodes, ".knownNodes")
	if write_err != nil {
		log.Fatalf("writeLines: %s", write_err)
	}
}

func gobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		//log.Panic(err)
	}

	return buff.Bytes()
}

func nodeIsKnown(addr string) bool {
	if nodes[addr] != "" {
		return true
	} else {
		return false
	}
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given file.
func writeLines(lines []string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}

		file.Close()
	}()

	w := bufio.NewWriter(file)
	if lines != nil {
		for _, line := range lines {
			if line == "" {
				continue
			}

			if len(line) < 6 {
				continue
			}

			cryptoNode := make([]byte, len(line))
			cryptoBlock.Encrypt(cryptoNode, []byte(line))
			fmt.Fprintln(w, string(cryptoNode))
		}
	}
	return w.Flush()
}

func ping_sort_address() {
	//fmt.Printf("[ping sort] node address count: %d\n", len(knownNodes))

	if len(knownNodes) > 1 {
		p := nodePings{}

		for _, node := range knownNodes {
			//if nodeAddress == node {
			//	continue
			//}

			str_ip4 := strings.Split(node, ":")
			pinger, err := ping.NewPinger(str_ip4[0])
			if err != nil {
				//panic(err)
				continue
			}

			pinger.SetPrivileged(true)

			pinger.Count = 1
			pinger.Timeout = 1 * time.Second
			pinger.Run() // block until finished
			pinger.SetPrivileged(false)
			stats := pinger.Statistics() // get send/receive/rtt stats

			var tmpNodePing nodePing

			tmpNodePing.node = node

			if stats.AvgRtt == 0 {
				tmpNodePing.pingTime = 100000000000
				//tmpNodePing.pingTime = 10000000
				//fmt.Printf("[ping sort] %s is deleted\n", str_ip4[0])

				continue
			} else {
				tmpNodePing.pingTime = stats.AvgRtt
			}

			//fmt.Printf("[ping sort] ping %s : %v\n", str_ip4[0], tmpNodePing.pingTime)

			p = append(p, tmpNodePing)

		}

		//fmt.Println("[ping sort] sort by ping")

		sort.Sort(ByPingTime{p})

		knownNodes = nil
		for _, node := range p {
			knownNodes = append(knownNodes, node.node)
			//fmt.Printf("ping sort node %d: %s\n", idx + 1, node.node)
		}
	}
}

func (nps nodePings) Len() int {
	return len(nps)
}

func (p ByPingTime) Less(i, j int) bool {
	return p.nodePings[i].pingTime < p.nodePings[j].pingTime
}

func (p ByPingTime) Swap(i, j int) {
	p.nodePings[i], p.nodePings[j] = p.nodePings[j], p.nodePings[i]
}
