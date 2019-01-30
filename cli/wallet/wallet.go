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

func showAccountInfo(wallet vault.Wallet, verbose bool) {
	const format = "%-34s  %s\n"
	account, _ := wallet.GetDefaultAccount()
	fmt.Printf(format, "Address", "Public Key")
	fmt.Printf(format, "-------", "----------")
	address, _ := account.ProgramHash.ToAddress()
	publicKey, _ := account.PublicKey.EncodePoint(true)
	fmt.Printf(format, address, BytesToHexString(publicKey))
	if verbose {
		fmt.Printf("\nPrivate Key\n-----------\n%s\n", BytesToHexString(account.PrivateKey))
	}
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
	// get password from the command line or from environment variable
	passwd := c.String("password")
	if passwd == "" {
		passwd = os.Getenv("NKN_WALLET_PASSWORD")
	}

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
			showAccountInfo(wallet, false)
		}
		return nil
	}

	// list wallet info
	if item := c.String("list"); item != "" {
		if item != "account" && item != "balance" && item != "verbose" && item != "nonce" {
			fmt.Fprintln(os.Stderr, "--list [account | balance | verbose | nonce]")
			os.Exit(1)
		} else {
			wallet, err := vault.OpenWallet(name, getPassword(passwd))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
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
					fmt.Fprintln(os.Stderr, err)
					return err
				}
				FormatOutput(resp)
			case "nonce":
				account, _ := wallet.GetDefaultAccount()
				address, _ := account.ProgramHash.ToAddress()
				resp, err := client.Call(Address(), "getnoncebyaddr", 0, map[string]interface{}{"address": address})
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
				Usage: "list wallet information [account, balance, verbose, nonce]",
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
