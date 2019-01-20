package errors

import (
	"fmt"

	"github.com/nknorg/nkn/util/log"
)

type ErrCoder interface {
	GetErrCode() ErrCode
}

type ErrCode int32

const (
	ErrNoCode                ErrCode = -2
	ErrNoError               ErrCode = 0
	ErrUnknown               ErrCode = -1
	ErrDuplicatedTx          ErrCode = 1
	ErrInputOutputTooLong    ErrCode = 45002
	ErrDuplicateInput        ErrCode = 45003
	ErrAssetPrecision        ErrCode = 45004
	ErrTransactionBalance    ErrCode = 45005
	ErrAttributeProgram      ErrCode = 45006
	ErrTransactionContracts  ErrCode = 45007
	ErrTransactionPayload    ErrCode = 45008
	ErrDoubleSpend           ErrCode = 45009
	ErrTxHashDuplicate       ErrCode = 45010
	ErrStateUpdaterVaild     ErrCode = 45011
	ErrSummaryAsset          ErrCode = 45012
	ErrXmitFail              ErrCode = 45013
	ErrNonOptimalSigChain    ErrCode = 45014
	ErrDuplicateName         ErrCode = 45015
	ErrMineReward            ErrCode = 45016
	ErrDuplicateSubscription ErrCode = 45017
	ErrSubscriptionLimit     ErrCode = 45018
	ErrDoNotPropagate        ErrCode = 45019
	ErrAlreadySubscribed     ErrCode = 45020
)

var ErrCode2Str = map[ErrCode]string{
	ErrNoCode:                "No error code",
	ErrNoError:               "Not an error",
	ErrUnknown:               "Unknown error",
	ErrDuplicatedTx:          "There are duplicated Transactions",
	ErrInputOutputTooLong:    "Transaction Inputs/Output too long",
	ErrDuplicateInput:        "Two inputs reference same UTXO",
	ErrAssetPrecision:        "The precision of asset is incorrect",
	ErrTransactionBalance:    "Input/output UTXO not equal",
	ErrAttributeProgram:      "CheckAttributeProgram to be implemented",
	ErrTransactionContracts:  "Contracts Error",
	ErrTransactionPayload:    "Invalidate transaction payload",
	ErrDoubleSpend:           "DoubleSpend check faild",
	ErrTxHashDuplicate:       "Duplicate transaction Hash",
	ErrStateUpdaterVaild:     "INTERNAL ERROR, ErrStateUpdaterVaild",
	ErrSummaryAsset:          "IssueAsset incorrect",
	ErrXmitFail:              "Xmit transaction failed",
	ErrNonOptimalSigChain:    "This SigChain is NOT optimal choice",
	ErrDuplicateName:         "Duplicate NameService operation in one block",
	ErrMineReward:            "Incorrect Mining reward",
	ErrDuplicateSubscription: "Duplicate subscription in one block",
	ErrSubscriptionLimit:     "Subscription limit exceeded in one block",
	ErrDoNotPropagate:        "Transaction should not be further propagated",
	ErrAlreadySubscribed:     "Client already subscribed to topic",
}

func (err ErrCode) Error() string {
	if s, ok := ErrCode2Str[err]; ok {
		return s
	}

	return fmt.Sprintf("Unknown error? Error code = %d", err)
}

func ErrerCode(err error) ErrCode {
	if err, ok := err.(ErrCoder); ok {
		return err.GetErrCode()
	}
	log.Errorf("Can't convert err %s to ErrCode", err)
	return ErrUnknown
}
