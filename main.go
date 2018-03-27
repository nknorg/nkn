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
	"nkn-core/net/httpjsonrpc"
	"nkn-core/net/httprestful"
	"nkn-core/net/httpwebsocket"
	"nkn-core/net/httpnodeinfo"
	"nkn-core/net/protocol"
	"os"
	"runtime"
	"time"
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
	httpjsonrpc.RegistRpcNode(noder)
	time.Sleep(20 * time.Second)
	noder.SyncNodeHeight()
	noder.WaitForFourPeersStart()
	noder.WaitForSyncBlkFinish()
	if protocol.SERVICENODENAME != config.Parameters.NodeType {
		log.Info("5. Start DBFT Services")
		dbftServices := dbft.NewDbftService(client, "logdbft", noder)
		httpjsonrpc.RegistDbftService(dbftServices)
		go dbftServices.Start()
		time.Sleep(5 * time.Second)
	}
	httpjsonrpc.Wallet = client
	log.Info("--Start the RPC interface")
	go httpjsonrpc.StartRPCServer()
	go httprestful.StartServer(noder)
	go httpwebsocket.StartServer(noder)
	if config.Parameters.HttpInfoStart {
		go httpnodeinfo.StartServer(noder)
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
