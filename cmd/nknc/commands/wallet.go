package commands

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nknorg/nkn/v2/api/httpjson/client"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/util/password"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/spf13/cobra"
)

// walletCmd represents the wallet command
var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "user wallet operation",
	Long:  "With nknc wallet, you could control your asset.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return walletAction()
	},
}

var (
	create         bool
	restore        string
	changepassword bool
	reset          bool
	listflag       string
)

func init() {
	rootCmd.AddCommand(walletCmd)

	walletCmd.Flags().BoolVarP(&create, "create", "c", false, "create wallet")
	walletCmd.Flags().StringVarP(&listflag, "list", "l", "", "list wallet information [account, balance, verbose, nonce]")
	walletCmd.Flags().StringVarP(&restore, "restore", "r", "", "restore wallet with [hex, bin] format PrivateKey")
	walletCmd.Flags().BoolVar(&changepassword, "changepassword", false, "change wallet password")
	walletCmd.Flags().BoolVar(&reset, "reset", false, "reset wallet")
	walletCmd.Flags().StringVarP(&walletFile, "name", "n", config.Parameters.WalletFile, "wallet file")
	walletCmd.Flags().StringVarP(&walletPassword, "password", "p", "", "wallet password")
}

func walletAction() error {
	// get password from the command line or from environment variable
	if walletPassword == "" {
		walletPassword = os.Getenv("NKN_WALLET_PASSWORD")
	}

	// create wallet
	if create {
		if common.FileExisted(walletFile) {
			return fmt.Errorf("CAUTION: '%s' already exists!\n", walletFile)
		}
		wallet, err := vault.NewWallet(walletFile, getConfirmedPassword(walletPassword))
		if err != nil {
			return err
		}
		showAccountInfo(wallet, false)
		return nil
	}

	var hexFmt bool
	switch format := restore; format {
	case "": // Not restore mode
		break
	case "hex":
		hexFmt = true
		fallthrough
	case "bin":
		if common.FileExisted(walletFile) {
			return fmt.Errorf("CAUTION: '%s' already exists!\n", walletFile)
		}

		key, err := password.GetPassword("Input you Secret Seed")
		if err != nil {
			return err
		}

		if hexFmt {
			if key, err = hex.DecodeString(string(key)); err != nil {
				return fmt.Errorf("Invalid hex. %v\n", err)
			}
		}
		wallet, err := vault.RestoreWallet(walletFile, GetPassword(walletPassword), key)
		if err != nil {
			return err
		}

		fmt.Printf("Restore %s wallet to %s\n", wallet.Address, walletFile)
		return nil
	default:
		return fmt.Errorf("--restore [hex | bin]")
	}

	// list wallet info
	if item := listflag; item != "" {
		if item != "account" && item != "balance" && item != "verbose" && item != "nonce" && item != "id" {
			return fmt.Errorf("--list [account | balance | verbose | nonce | id]")
		} else {
			wallet, err := vault.OpenWallet(walletFile, GetPassword(walletPassword))
			if err != nil {
				return err
			}
			var vbs bool
			switch item {
			case "verbose":
				vbs = true
				fallthrough
			case "account":
				showAccountInfo(wallet, vbs)
			case "balance":
				account, _ := wallet.GetDefaultAccount()
				address, _ := account.ProgramHash.ToAddress()
				resp, err := client.Call(Address(), "getbalancebyaddr", 0, map[string]interface{}{"address": address})
				if err != nil {
					return err
				}
				FormatOutput(resp)
			case "nonce":
				account, _ := wallet.GetDefaultAccount()
				address, _ := account.ProgramHash.ToAddress()
				resp, err := client.Call(Address(), "getnoncebyaddr", 0, map[string]interface{}{"address": address})
				if err != nil {
					return err
				}
				FormatOutput(resp)
			case "id":
				account, _ := wallet.GetDefaultAccount()
				publicKey := account.PubKey()
				pk := hex.EncodeToString(publicKey)
				resp, err := client.Call(Address(), "getid", 0, map[string]interface{}{"publickey": pk})
				if err != nil {
					return err
				}
				FormatOutput(resp)
			}
		}
		return nil
	}

	// change password
	if changepassword {
		fmt.Printf("Wallet File: '%s'\n", walletFile)
		passwd, err := password.GetPassword("")
		if err != nil {
			return err
		}
		wallet, err := vault.OpenWallet(walletFile, passwd)
		if err != nil {
			return err
		}
		fmt.Println("# input new password #")
		newPassword, err := password.GetConfirmedPassword()
		if err != nil {
			return err
		}
		err = wallet.ChangePassword([]byte(passwd), newPassword)
		if err != nil {
			return err
		}
		fmt.Println("password changed")

		return nil
	}

	return nil
}

func showAccountInfo(wallet *vault.Wallet, verbose bool) {
	const format = "%-37s  %s\n"
	account, _ := wallet.GetDefaultAccount()
	fmt.Printf(format, "Address", "Public Key")
	fmt.Printf(format, "-------", "----------")
	address, _ := account.ProgramHash.ToAddress()
	publicKey := account.PublicKey
	fmt.Printf(format, address, hex.EncodeToString(publicKey))
	if verbose {
		fmt.Printf("\nSecret Seed\n-----------\n%s\n", hex.EncodeToString(crypto.GetSeedFromPrivateKey(account.PrivateKey)))
	}
}

func getConfirmedPassword(passwd string) []byte {
	var tmp []byte
	var err error
	if passwd != "" {
		tmp = []byte(passwd)
	} else {
		tmp, err = password.GetConfirmedPassword()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	return tmp
}
