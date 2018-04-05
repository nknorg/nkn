package ising

import (
	"fmt"
	"math/rand"

	."nkn-core/common"
	"nkn-core/core/ledger"
	"nkn-core/core/transaction"
	"nkn-core/events"
	"nkn-core/net"
	"nkn-core/net/message"
	"nkn-core/wallet"
	"nkn-core/core/transaction/payload"
	"nkn-core/core/contract/program"
	"nkn-core/crypto"
)

const (
	TxnAmountToBePackaged = 1024
)

type Ising struct {
	wallet               wallet.Wallet             // local account
	role                 Bitmap                    // node role
	state                Bitmap                    // consensus state
	localNode            net.Neter                 // local node
	txnCollector         *transaction.TxnCollector // collect transaction from where
	blockCache           *BlockCache               // blocks waiting for voting
	consensusMsgReceived events.Subscriber         // consensus events listening
}

func New(wallet wallet.Wallet, node net.Neter) *Ising {
	var role Bitmap

	role.SetBit(BlockVoter)
	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil
	}
	encPubKey, err := account.PublicKey.EncodePoint(true)
	if err != nil {
		return nil
	}
	confPubKey, err := ledger.StandbyBookKeepers[0].EncodePoint(true)
	if IsEqualBytes(encPubKey, confPubKey) {
		role.SetBit(BlockProposer)
	}

	ising := &Ising{
		role:         role,
		state:        InitialState,
		wallet:       wallet,
		localNode:    node,
		txnCollector: transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged),
		blockCache:   NewCache(),
	}

	return ising
}

func (p *Ising) Start() error {
	p.consensusMsgReceived = p.localNode.GetEvent("consensus").Subscribe(events.EventConsensusMsgReceived, p.ConsensusMsgReceived)
	if p.role.HasBit(BlockProposer) {
		fmt.Println("I am block proposer")
		block, err := p.BuildBlock()
		if err != nil {
			return err
		}
		err = p.blockCache.AddBlockToCache(block)
		if err != nil {
			return err
		}
		blockFlooding := &BlockFlooding{
			block: block,
		}
		payload, err := BuildIsingPayload(blockFlooding)
		if err != nil {
			return err
		}
		err = p.localNode.Xmit(payload)
		if err != nil {
			return err
		}
		p.state.SetBit(FloodingFinished)
	}

	return nil
}

func  CreateBookkeepingTransaction() *transaction.Transaction {
	bookKeepingPayload := &payload.BookKeeping{
		Nonce: rand.Uint64(),
	}
	return &transaction.Transaction{
		TxType:         transaction.BookKeeping,
		PayloadVersion: payload.BookKeepingPayloadVersion,
		Payload:        bookKeepingPayload,
		Attributes:     []*transaction.TxAttribute{},
		UTXOInputs:     []*transaction.UTXOTxInput{},
		Outputs:        []*transaction.TxOutput{},
		Programs:       []*program.Program{},
	}
}

func (p *Ising) BuildBlock() (*ledger.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []Uint256
	coinbase := CreateBookkeepingTransaction()
	txnList = append(txnList, coinbase)
	txnHashList = append(txnHashList, coinbase.Hash())
	txns := p.txnCollector.Collect()
	for txnHash, txn := range txns {
		txnList = append(txnList, txn)
		txnHashList = append(txnHashList, txnHash)
	}
	txnRoot, err := crypto.ComputeRoot(txnHashList)
	if err != nil {
		return nil, err
	}
	header := &ledger.Header{
		TransactionsRoot: txnRoot,
		Program: &program.Program{
			Code:      []byte{},
			Parameter: []byte{},
		},
	}
	block := &ledger.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}

func (p *Ising) ConsensusMsgReceived(v interface{}) {
	if payload, ok := v.(*message.IsingPayload); ok {
		isingMsg, err := RecoverFromIsingPayload(payload.PayloadData)
		if err != nil {
			fmt.Println("Deserialization of ising message error")
		}
		switch isingMsg.(type) {
		case *BlockFlooding:
			fmt.Println("receive block flooding")
		case *BlockProposal:
			fmt.Println("receive block proposal")
		case *BlockRequest:
			fmt.Println("receive block request")
		case *BlockVote:
			fmt.Println("receive block vote")
		}
	}
}


