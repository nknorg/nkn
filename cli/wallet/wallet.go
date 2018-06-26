package wallet

import (
	"fmt"
	"os"

	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"

	"github.com/urfave/cli"
)

func showAccountInfo(wallet vault.Wallet) {
	account, _ := wallet.GetDefaultAccount()
	fmt.Println("Address\t\t\t\t Public Key")
	fmt.Println("-------\t\t\t\t ----------")
	address, _ := account.ProgramHash.ToAddress()
	publicKey, _ := account.PublicKey.EncodePoint(true)
	fmt.Printf("%s %s\n", address, BytesToHexString(publicKey))
}

func getPassword(passwd string) []byte {
	var tmp []byte
	var err error
	if passwd != "" {
		tmp = []byte(passwd)
	} else {
		tmp, err = password.GetPassword()
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
	// wallet name is wallet.dat by default
	name := c.String("name")
	if name == "" {
		fmt.Fprintln(os.Stderr, "invalid wallet name")
		os.Exit(1)
	}
	passwd := c.String("password")

	// create wallet
	if c.Bool("create") {
		if FileExisted(name) {
			fmt.Printf("CAUTION: '%s' already exists!\n", name)
			os.Exit(1)
		} else {
			wallet, err := vault.NewWallet(name, getConfirmedPassword(passwd), true)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			showAccountInfo(wallet)
		}
		return nil
	}

	// list wallet info
	if item := c.String("list"); item != "" {
		if item != "account" && item != "balance" && item != "verbose" {
			fmt.Fprintln(os.Stderr, "--list [account | balance | verbose]")
			os.Exit(1)
		} else {
			wallet, err := vault.OpenWallet(name, getPassword(passwd))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			switch item {
			case "account":
				showAccountInfo(wallet)
			case "balance":
				resp, err := client.Call(Address(), "getbalance", 0, map[string]interface{}{})
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return err
				}
				FormatOutput(resp)
			}
		}
		return nil
	}

	// change password
	if c.Bool("changepassword") {
		fmt.Printf("Wallet File: '%s'\n", name)
		passwd, _ := password.GetPassword()
		wallet, err := vault.OpenWallet(name, passwd)
		if err != nil {
			os.Exit(1)
		}
		fmt.Println("# input new password #")
		newPassword, _ := password.GetConfirmedPassword()
		if ok := wallet.ChangePassword([]byte(passwd), newPassword); !ok {
			fmt.Fprintln(os.Stderr, "failed to change password")
			os.Exit(1)
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
				Usage: "list wallet information [account, balance, verbose]",
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
				Value: vault.WalletFileName,
			},
			cli.StringFlag{
				Name:  "password, p",
				Usage: "wallet password",
			},
		},
		Action: walletAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "wallet")
			return cli.NewExitError("", 1)
		},
	}
}
