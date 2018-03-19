package main

import (
	"nkn-core/account"
	"nkn-core/common/config"
	"nkn-core/common/log"
	"nkn-core/core/ledger"
	"nkn-core/core/store"
	"nkn-core/crypto"
	"nkn-core/net"
	"nkn-core/net/httpjsonrpc"
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
	var acct *account.Account
	var blockChain *ledger.Blockchain
	var err error
	var noder protocol.Noder
	log.Trace("Node version: ", config.Version)

	ledger.DefaultLedger = new(ledger.Ledger)
	ledger.DefaultLedger.Store, err = store.NewLedgerStore()
	defer ledger.DefaultLedger.Store.Close()
	if err != nil {
		log.Fatal("open LedgerStore err:", err)
	}
	crypto.SetAlg(config.Parameters.EncryptAlg)

	log.Info("3. BlockChain init")
	blockChain, err = ledger.NewBlockchainWithGenesisBlock()
	if err != nil {
		log.Fatal(err, "  BlockChain generate failed")
	}
	ledger.DefaultLedger.Blockchain = blockChain

	log.Info("4. Start the P2P networks")
	client := account.GetClient()
	if client == nil {
		log.Fatal("Can't get local account.")
	}
	acct, err = client.GetDefaultAccount()
	if err != nil {
		log.Fatal(err)
	}
	noder = net.StartProtocol(acct.PublicKey)
	httpjsonrpc.RegistRpcNode(noder)
	time.Sleep(20 * time.Second)
	//noder.SyncNodeHeight()
	noder.WaitForSyncBlkFinish()

	log.Info("--Start the RPC interface")
	go httpjsonrpc.StartRPCServer()

	for {
		log.Trace("BlockHeight = ", ledger.DefaultLedger.Blockchain.BlockHeight)
		isNeedNewFile := log.CheckIfNeedNewFile()
		if isNeedNewFile == true {
			log.ClosePrintLog()
			log.Init(log.Path, os.Stdout)
		}
	}
}
