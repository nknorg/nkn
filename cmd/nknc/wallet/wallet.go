package wallet

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nknorg/nkn/v2/api/httpjson/client"
	nknc "github.com/nknorg/nkn/v2/cmd/nknc/common"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/util/password"
	"github.com/nknorg/nkn/v2/vault"

	"github.com/urfave/cli"
)

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

func getPassword(passwd string) []byte {
	var tmp []byte
	var err error
	if passwd != "" {
		tmp = []byte(passwd)
	} else {
		tmp, err = password.GetPassword("")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	return tmp
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

func walletAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}
	// wallet file name
	name := c.String("name")
	if name == "" {
		return fmt.Errorf("invalid wallet name")
	}
	// get password from the command line or from environment variable
	passwd := c.String("password")
	if passwd == "" {
		passwd = os.Getenv("NKN_WALLET_PASSWORD")
	}

	// create wallet
	if c.Bool("create") {
		if common.FileExisted(name) {
			return fmt.Errorf("CAUTION: '%s' already exists!\n", name)
		}
		wallet, err := vault.NewWallet(name, getConfirmedPassword(passwd))
		if err != nil {
			return err
		}
		showAccountInfo(wallet, false)
		return nil
	}

	var hexFmt bool
	switch format := c.String("restore"); format {
	case "": // Not restore mode
		break
	case "hex":
		hexFmt = true
		fallthrough
	case "bin":
		if common.FileExisted(name) {
			return fmt.Errorf("CAUTION: '%s' already exists!\n", name)
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
		wallet, err := vault.RestoreWallet(name, getPassword(passwd), key)
		if err != nil {
			return err
		}

		fmt.Printf("Restore %s wallet to %s\n", wallet.Address, name)
		return nil
	default:
		return fmt.Errorf("--restore [hex | bin]")
	}

	// list wallet info
	if item := c.String("list"); item != "" {
		if item != "account" && item != "balance" && item != "verbose" && item != "nonce" && item != "id" {
			return fmt.Errorf("--list [account | balance | verbose | nonce | id]")
		} else {
			wallet, err := vault.OpenWallet(name, getPassword(passwd))
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
				resp, err := client.Call(nknc.Address(), "getbalancebyaddr", 0, map[string]interface{}{"address": address})
				if err != nil {
					return err
				}
				nknc.FormatOutput(resp)
			case "nonce":
				account, _ := wallet.GetDefaultAccount()
				address, _ := account.ProgramHash.ToAddress()
				resp, err := client.Call(nknc.Address(), "getnoncebyaddr", 0, map[string]interface{}{"address": address})
				if err != nil {
					return err
				}
				nknc.FormatOutput(resp)
			case "id":
				account, _ := wallet.GetDefaultAccount()
				publicKey := account.PubKey()
				pk := hex.EncodeToString(publicKey)
				resp, err := client.Call(nknc.Address(), "getid", 0, map[string]interface{}{"publickey": pk})
				if err != nil {
					return err
				}
				nknc.FormatOutput(resp)
			}
		}
		return nil
	}

	// change password
	if c.Bool("changepassword") {
		fmt.Printf("Wallet File: '%s'\n", name)
		passwd, err := password.GetPassword("")
		if err != nil {
			return err
		}
		wallet, err := vault.OpenWallet(name, passwd)
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

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "wallet",
		Usage:       "user wallet operation",
		Description: "With nknc wallet, you could control your asset.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "create, c",
				Usage: "create wallet",
			},
			cli.StringFlag{
				Name:  "list, l",
				Usage: "list wallet information [account, balance, verbose, nonce]",
			},
			cli.StringFlag{
				Name:  "restore, r",
				Usage: "restore wallet with [hex, bin] format PrivateKey",
			},
			cli.BoolFlag{
				Name:  "changepassword",
				Usage: "change wallet password",
			},
			cli.BoolFlag{
				Name:  "reset",
				Usage: "reset wallet",
			},
			cli.StringFlag{
				Name:  "name, n",
				Usage: "wallet name",
				Value: config.Parameters.WalletFile,
			},
			cli.StringFlag{
				Name:  "password, p",
				Usage: "wallet password",
			},
		},
		Action: walletAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			nknc.PrintError(c, err, "wallet")
			return cli.NewExitError("", 1)
		},
	}
}
