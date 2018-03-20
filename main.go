package main

import (
	"log"
	"runtime"
	"time"

	"math/rand"
	. "nkn-core/common"
	"nkn-core/common/config"
	"nkn-core/core/ledger"
	"nkn-core/core/store"
	"nkn-core/crypto"
)

const (
	DefaultMultiCoreNum = 4
)

func init() {
	//log.Init(log.Path, log.Stdout)
	crypto.SetAlg(config.Parameters.EncryptAlg)
	var coreNum int
	if config.Parameters.MultiCoreNum > DefaultMultiCoreNum {
		coreNum = int(config.Parameters.MultiCoreNum)
	} else {
		coreNum = DefaultMultiCoreNum
	}
	runtime.GOMAXPROCS(coreNum)
}

func main() {
	log.Println("Node version: ", config.Version)
	var err error

	ledger.DefaultLedger = new(ledger.Ledger)
	ledger.DefaultLedger.Store, err = store.NewLedgerStore()
	defer ledger.DefaultLedger.Store.Close()
	if err != nil {
		log.Fatalln("new store error")
	}
	ledger.DefaultLedger.Blockchain, err = ledger.NewBlockchainWithGenesisBlock()
	if err != nil {
		log.Fatalln("new blockchain error")
	}

	preHash := Uint256{}
	t := time.NewTicker(time.Second)
	for {
		select {
		case <-t.C:
			b := &ledger.Block{
				Header: &ledger.BlockHeader{
					Version:          0,
					PrevBlockHash:    ledger.DefaultLedger.Blockchain.CurrentBlockHash(),
					Timestamp:        uint32(time.Now().Unix()),
					TransactionsRoot: Uint256{},
					Height:           ledger.DefaultLedger.Blockchain.CurrentBlockHeight() + 1,
					Nonce:            rand.Uint32(),
				},
			}
			preHash = b.Hash()
			err := ledger.DefaultLedger.Blockchain.AddBlock(b)
			if err != nil {
				log.Fatalln(err)
			}
			log.Println("Add block: ", ToHexString(preHash.ToArray()))
		}
	}

	select {}
	/*
		var acct *account.Account
		log.Info("4. Start the P2P networks")
		client := account.GetClient()
		if client == nil {
			log.Fatal("Can't get local account.")
		}
		acct, err = client.GetDefaultAccount()
		if err != nil {
			log.Fatal(err)
		}
		var noder protocol.Noder
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
	*/
}
