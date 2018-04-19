package main

import (
	"errors"
	"flag"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/nknorg/nkn/consensus/dbft"
	"github.com/nknorg/nkn/consensus/ising"
	"github.com/nknorg/nkn/core/ledger"
	"github.com/nknorg/nkn/core/transaction"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/db"
	"github.com/nknorg/nkn/net"
	"github.com/nknorg/nkn/net/chord"
	"github.com/nknorg/nkn/net/protocol"
	_ "github.com/nknorg/nkn/por" // for testing sigchain of PoR feature
	"github.com/nknorg/nkn/rpc/httpjson"
	"github.com/nknorg/nkn/rpc/httprestful"
	"github.com/nknorg/nkn/test/monitor"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/log"
	"github.com/nknorg/nkn/wallet"
	"github.com/nknorg/nkn/ws"
)

func init() {
	log.Init(log.Path, log.Stdout)
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())
}

func nknMain() error {
	var name = flag.String("test", "value", "usage")
	var numNode int
	flag.IntVar(&numNode, "numNode", 1, "usage")
	flag.Parse()

	// Start the Chord ring testing process
	if len(os.Args) != 1 {
		//flag.PrintDefaults()
		if *name == "create" {
			go chord.CreateNet()
		} else if *name == "join" {
			go chord.JoinNet()
		}
		for {
			time.Sleep(20 * time.Second)
		}
	}

	var acct *wallet.Account
	var blockChain *ledger.Blockchain
	var err error
	var noder protocol.Noder
	log.Trace("Node version: ", config.Version)

	// TODO remove the bookkeepers limitation
	if len(config.Parameters.BookKeepers) < wallet.DefaultBookKeeperCount {
		log.Fatal("At least ", wallet.DefaultBookKeeperCount, " BookKeepers should be set at config.json")
		return errors.New("BookKeepers should be set at config.json")
	}

	log.Info("0. Loading the Ledger")
	ledger.DefaultLedger = new(ledger.Ledger)
	ledger.DefaultLedger.Store, err = db.NewLedgerStore()
	defer ledger.DefaultLedger.Store.Close()
	if err != nil {
		log.Fatal("open LedgerStore err:", err)
		return err
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
	client := wallet.GetWallet()
	if client == nil {
		log.Fatal("Can't get local account.")
		return errors.New("Can't get local account")

	}
	acct, err = client.GetDefaultAccount()
	if err != nil {
		log.Fatal(err)
		return err
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
		ising.StartIsingConsensus(acct, noder)
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

}

func main() {
	// Call the nknMain so the defers will be executed in the case of a graceful shutdown.
	if err := nknMain(); err != nil {
		os.Exit(1)
	}
}
