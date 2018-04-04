package main

import (
	"nkn-core/wallet"
	"nkn-core/common/config"
	"nkn-core/common/log"
	"nkn-core/consensus/dbft"
	"nkn-core/core/ledger"
	"nkn-core/core/store/ChainStore"
	"nkn-core/core/transaction"
	"nkn-core/crypto"
	"nkn-core/net"
	"nkn-core/rpc/httpjson"
	"nkn-core/rpc/httprestful"
	"nkn-core/ws"
	"nkn-core/test/monitor"
	"nkn-core/net/protocol"
	_"nkn-core/core/sigchain" // for testing sigchain package
	"os"
	"runtime"
	"time"
	"math/rand"
	"nkn-core/consensus/ising"
)

const (
	DefaultMultiCoreNum = 4
)

func init() {
	log.Init(log.Path, log.Stdout)
	var coreNum int
	if config.Parameters.MultiCoreNum > DefaultMultiCoreNum {
		coreNum = int(config.Parameters.MultiCoreNum)
	} else {
		coreNum = DefaultMultiCoreNum
	}
	log.Debug("The Core number is ", coreNum)
	runtime.GOMAXPROCS(coreNum)
	rand.Seed(time.Now().UnixNano())
}

func main() {
	var acct *wallet.Account
	var blockChain *ledger.Blockchain
	var err error
	var noder protocol.Noder
	log.Trace("Node version: ", config.Version)

	if len(config.Parameters.BookKeepers) < wallet.DefaultBookKeeperCount {
		log.Fatal("At least ", wallet.DefaultBookKeeperCount, " BookKeepers should be set at config.json")
		os.Exit(1)
	}

	log.Info("0. Loading the Ledger")
	ledger.DefaultLedger = new(ledger.Ledger)
	ledger.DefaultLedger.Store, err = ChainStore.NewLedgerStore()
	defer ledger.DefaultLedger.Store.Close()
	if err != nil {
		log.Fatal("open LedgerStore err:", err)
		os.Exit(1)
	}
	ledger.DefaultLedger.Store.InitLedgerStore(ledger.DefaultLedger)
	transaction.TxStore = ledger.DefaultLedger.Store
	crypto.SetAlg(config.Parameters.EncryptAlg)
	ledger.StandbyBookKeepers = wallet.GetBookKeepers()

	log.Info("2. BlockChain init")
	blockChain, err = ledger.NewBlockchainWithGenesisBlock(ledger.StandbyBookKeepers)
	if err != nil {
		log.Fatal(err, "  BlockChain generate failed")
	}
	ledger.DefaultLedger.Blockchain = blockChain

	log.Info("3. Start the P2P networks")
	client := wallet.GetClient()
	if client == nil {
		log.Fatal("Can't get local account.")
		goto ERROR
	}
	acct, err = client.GetDefaultAccount()
	if err != nil {
		log.Fatal(err)
		goto ERROR
	}
	noder = net.StartProtocol(acct.PublicKey)
	httpjson.RegistRpcNode(noder)
	time.Sleep(10 * time.Second)
	noder.SyncNodeHeight()
	noder.WaitForFourPeersStart()
	noder.WaitForSyncBlkFinish()
	if protocol.SERVICENODENAME != config.Parameters.NodeType {
		//log.Info("5. Start DBFT Services")
		//dbftServices := dbft.NewDbftService(client, "logdbft", noder)
		//go dbftServices.Start()
		consensus := ising.New(client, noder)
		consensus.Start()
		time.Sleep(5 * time.Second)
	}
	httpjson.Wallet = client
	log.Info("--Start the RPC interface")
	go httpjson.StartRPCServer()
	go httprestful.StartServer(noder)
	go ws.StartServer(noder)
	if config.Parameters.HttpInfoStart {
		go monitor.StartServer(noder)
	}


	for {
		time.Sleep(dbft.GenBlockTime)
		log.Trace("BlockHeight = ", ledger.DefaultLedger.Blockchain.BlockHeight)
		isNeedNewFile := log.CheckIfNeedNewFile()
		if isNeedNewFile == true {
			log.ClosePrintLog()
			log.Init(log.Path, os.Stdout)
		}
	}

ERROR:
	os.Exit(1)
}
