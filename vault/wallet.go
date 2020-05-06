package vault

import (
	"errors"
	"fmt"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	serviceConfig "github.com/nknorg/nkn/dashboard/config"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/program"
	"github.com/nknorg/nkn/signature"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/password"
)

type Wallet struct {
	*WalletStore
	PasswordHash []byte
	account      *Account
	contract     *program.ProgramContext
}

func NewWallet(path string, password []byte) (*Wallet, error) {
	account, err := NewAccount()
	if err != nil {
		return nil, err
	}

	walletData, err := NewWalletData(account, password, nil, nil, nil, 0, 0, 0)
	if err != nil {
		return nil, err
	}

	walletStore, err := NewWalletStore(path, walletData)
	if err != nil {
		return nil, err
	}

	contract, err := program.CreateSignatureProgramContext(account.PubKey())
	if err != nil {
		return nil, err
	}

	err = walletStore.Save()
	if err != nil {
		return nil, err
	}

	return &Wallet{
		WalletStore:  walletStore,
		PasswordHash: crypto.PasswordHash(password),
		account:      account,
		contract:     contract,
	}, nil
}

func OpenWallet(path string, password []byte) (*Wallet, error) {
	walletStore, err := LoadWalletStore(path)
	if err != nil {
		return nil, err
	}

	walletData := walletStore.WalletData

	if walletData.Version < MinCompatibleWalletVersion || walletData.Version > MaxCompatibleWalletVersion {
		return nil, fmt.Errorf("invalid wallet version %v, should be between %v and %v", walletData.Version, MinCompatibleWalletVersion, MaxCompatibleWalletVersion)
	}

	account, err := walletData.DecryptAccount(password)
	if err != nil {
		return nil, err
	}

	address, err := account.ProgramHash.ToAddress()
	if err != nil {
		return nil, err
	}

	if address != walletData.Address {
		return nil, errors.New("wrong password")
	}

	contract, err := program.CreateSignatureProgramContext(account.PubKey())
	if err != nil {
		return nil, err
	}

	return &Wallet{
		WalletStore:  walletStore,
		PasswordHash: crypto.PasswordHash(password),
		account:      account,
		contract:     contract,
	}, nil
}

func (w *Wallet) GetDefaultAccount() (*Account, error) {
	if w.account == nil {
		return nil, errors.New("account error")
	}
	return w.account, nil
}

func (w *Wallet) Sign(txn *transaction.Transaction) error {
	contract, err := w.GetContract()
	if err != nil {
		return fmt.Errorf("cannot get contract from wallet: %v", err)
	}

	account, err := w.GetDefaultAccount()
	if err != nil {
		return fmt.Errorf("no available account in wallet: %v", account)
	}

	signature, err := signature.SignBySigner(txn, account)
	if err != nil {
		return err
	}

	program := contract.NewProgram(signature)

	txn.SetPrograms([]*pb.Program{program})

	return nil
}

func (w *Wallet) VerifyPassword(password []byte) error {
	return w.WalletStore.WalletData.VerifyPassword(password)
}

func (w *Wallet) ChangePassword(oldPassword, newPassword []byte) error {
	account, err := w.DecryptAccount(oldPassword)
	if err != nil {
		return err
	}

	address, err := account.ProgramHash.ToAddress()
	if err != nil {
		return err
	}

	if address != w.Address {
		return errors.New("wrong password")
	}

	w.WalletData, err = NewWalletData(account, newPassword, nil, nil, nil, 0, 0, 0)
	if err != nil {
		return err
	}

	err = w.Save()
	if err != nil {
		return err
	}

	return nil
}

func (w *Wallet) GetContract() (*program.ProgramContext, error) {
	if w.contract == nil {
		return nil, errors.New("contract error")
	}

	return w.contract, nil
}

func GetWallet() (*Wallet, error) {
	walletFileName := config.Parameters.WalletFile
	if !common.FileExisted(walletFileName) {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_WALLET_FILE
		return nil, fmt.Errorf("wallet file %s does not exist, please create a wallet using nknc", walletFileName)
	}
	serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_WALLET_FILE

	passwd, err := password.GetAccountPassword()
	defer common.ClearBytes(passwd)
	if err != nil {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_PASSWORD
		return nil, fmt.Errorf("get password error: %v", err)
	}
	serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_PASSWORD

	if (serviceConfig.Status&serviceConfig.SERVICE_STATUS_NO_WALLET_FILE) != 0 && !config.Parameters.AllowEmptyBeneficiaryAddress && config.Parameters.BeneficiaryAddr == "" {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_BENEFICIARY
		return nil, fmt.Errorf("wait for set beneficiary address")
	}
	serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_BENEFICIARY

	w, err := OpenWallet(walletFileName, passwd)
	if err != nil {
		serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_NO_PASSWORD
		return nil, fmt.Errorf("open wallet error: %v", err)
	}
	serviceConfig.Status = serviceConfig.Status &^ serviceConfig.SERVICE_STATUS_NO_PASSWORD
	serviceConfig.Status = serviceConfig.Status | serviceConfig.SERVICE_STATUS_RUNNING
	return w, nil
}
