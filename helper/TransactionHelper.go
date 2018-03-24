package helper

import (
	"errors"
	"fmt"
	"sort"

	"nkn-core/account"
	. "nkn-core/common"
	. "nkn-core/core/asset"
	"nkn-core/core/contract"
	"nkn-core/core/transaction"
)

type BatchOut struct {
	Address string
	Value   string
}

type sortedCoinsItem struct {
	input *transaction.UTXOTxInput
	coin  *account.Coin
}

// sortedCoins used for spend minor coins first
type sortedCoins []*sortedCoinsItem

func (sc sortedCoins) Len() int      { return len(sc) }
func (sc sortedCoins) Swap(i, j int) { sc[i], sc[j] = sc[j], sc[i] }
func (sc sortedCoins) Less(i, j int) bool {
	if sc[i].coin.Output.Value > sc[j].coin.Output.Value {
		return false
	} else {
		return true
	}
}

func sortCoinsByValue(coins map[*transaction.UTXOTxInput]*account.Coin, addrtype account.AddressType) sortedCoins {
	var coinList sortedCoins
	for in, c := range coins {
		if c.AddressType == addrtype {
			tmp := &sortedCoinsItem{
				input: in,
				coin:  c,
			}
			coinList = append(coinList, tmp)
		}
	}
	sort.Sort(coinList)
	return coinList
}

func MakeRegTransaction(wallet account.Client, name string, value string) (*transaction.Transaction, error) {
	admin, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	issuer := admin
	asset := &Asset{name, name, byte(MaxPrecision), AssetType(Token), UTXO}
	transactionContract, err := contract.CreateSignatureContract(admin.PubKey())
	if err != nil {
		fmt.Println("CreateSignatureContract failed")
		return nil, err
	}
	fixedValue, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewRegisterAssetTransaction(asset, fixedValue, issuer.PubKey(), transactionContract.ProgramHash)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())


	return txn, nil
}

func MakeIssueTransaction(wallet account.Client, assetID Uint256, address string, value string) (*transaction.Transaction, error) {
	programHash, err := ToScriptHash(address)
	if err != nil {
		return nil, err
	}
	fixedValue, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}
	issueTxOutput := &transaction.TxOutput{
		AssetID:     assetID,
		Value:       fixedValue,
		ProgramHash: programHash,
	}
	outputs := []*transaction.TxOutput{issueTxOutput}
	txn, err := transaction.NewIssueAssetTransaction(outputs)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeTransferTransaction(wallet account.Client, assetID Uint256, batchOut ...BatchOut) (*transaction.Transaction, error) {
	//TODO: check if being transferred asset is System Token(IPT)
	outputNum := len(batchOut)
	if outputNum == 0 {
		return nil, errors.New("nil outputs")
	}

	// get main account which is used to receive changes
	mainAccount, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	perOutputFee := Fixed64(0)
	var expected Fixed64
	input := []*transaction.UTXOTxInput{}
	output := []*transaction.TxOutput{}
	// construct transaction outputs
	for _, o := range batchOut {
		outputValue, err := StringToFixed64(o.Value)
		if err != nil {
			return nil, err
		}
		if outputValue <= perOutputFee {
			return nil, errors.New("token is not enough for transaction fee")
		}
		expected += outputValue
		address, err := ToScriptHash(o.Address)
		if err != nil {
			return nil, errors.New("invalid address")
		}
		tmp := &transaction.TxOutput{
			AssetID:     assetID,
			Value:       outputValue - perOutputFee,
			ProgramHash: address,
		}
		output = append(output, tmp)
	}

	// construct transaction inputs and changes
	coins := wallet.GetCoins()
	sorted := sortCoinsByValue(coins, account.SingleSign)
	for _, coinItem := range sorted {
		if coinItem.coin.Output.AssetID == assetID {
			input = append(input, coinItem.input)
			if coinItem.coin.Output.Value > expected {
				changes := &transaction.TxOutput{
					AssetID:     assetID,
					Value:       coinItem.coin.Output.Value - expected,
					ProgramHash: mainAccount.ProgramHash,
				}
				// if any, the changes output of transaction will be the last one
				output = append(output, changes)
				expected = 0
				break
			} else if coinItem.coin.Output.Value == expected {
				expected = 0
				break
			} else if coinItem.coin.Output.Value < expected {
				expected = expected - coinItem.coin.Output.Value
			}
		}
	}
	if expected > 0 {
		return nil, errors.New("token is not enough")
	}

	// construct transaction
	txn, err := transaction.NewTransferAssetTransaction(input, output)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}
