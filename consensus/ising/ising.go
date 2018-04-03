package ising

import (
	. "nkn-core/common"
	"nkn-core/core/ledger"
	"nkn-core/core/transaction"
	"nkn-core/crypto"
	"nkn-core/net"
	"nkn-core/wallet"
	"fmt"
)

const (
	TxnAmountToBePackaged = 1024
)

type Ising struct {
	wallet       wallet.Wallet             // local account
	role         Role                      // proposer or voter
	localNode    net.Neter                 // local node
	txnCollector *transaction.TxnCollector // collect transaction from where
}

func New(wallet wallet.Wallet, node net.Neter) *Ising {
	var role Role
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
		role = BlockProposer
	} else {
		role = BlockVoter
	}
	ising := &Ising{
		role:         role,
		wallet:       wallet,
		localNode:    node,
		txnCollector: transaction.NewTxnCollector(node.GetTxnPool(), TxnAmountToBePackaged),
	}

	return ising
}

func (p *Ising) Start() error {
	switch p.role {
	case BlockProposer:
		fmt.Println("I am block proposer")
	case BlockVoter:
		fmt.Println("I am block voter")
	}

	return nil
}

func (p *Ising) BuildBlock() (*ledger.Block, error) {
	var txnList []*transaction.Transaction
	var txnHashList []Uint256

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
	}
	block := &ledger.Block{
		Header:       header,
		Transactions: txnList,
	}

	return block, nil
}
