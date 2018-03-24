package wallet

import (
	"fmt"
	"os"
	"strconv"
	"nkn-core/account"
	. "nkn-core/cli/common"
	. "nkn-core/common"
	"nkn-core/common/password"
	"github.com/urfave/cli"
)

func showAccountsInfo(wallet account.Client) {
	accounts := wallet.GetAccounts()
	fmt.Println(" ID   Address\t\t\t\t Public Key")
	fmt.Println("----  -------\t\t\t\t ----------")
	for i, account := range accounts {
		address, _ := account.ProgramHash.ToAddress()
		publicKey, _ := account.PublicKey.EncodePoint(true)
		fmt.Printf("%4s  %s %s\n", strconv.Itoa(i), address, BytesToHexString(publicKey))
	}
}

func showBalancesInfo(wallet account.Client) {
	coins := wallet.GetCoins()
	assets := make(map[Uint256]Fixed64)
	for _, out := range coins {
		if out.AddressType == account.SingleSign {
			if _, ok := assets[out.Output.AssetID]; !ok {
				assets[out.Output.AssetID] = out.Output.Value
			} else {
				assets[out.Output.AssetID] += out.Output.Value
			}
		}
	}
	if len(assets) == 0 {
		fmt.Println("no assets")
		return
	}
	fmt.Println(" ID   Asset ID\t\t\t\t\t\t\t\tAmount")
	fmt.Println("----  --------\t\t\t\t\t\t\t\t------")
	i := 0
	for id, amount := range assets {
		fmt.Printf("%4s  %s  %v\n", strconv.Itoa(i), BytesToHexString(id.ToArrayReverse()), amount)
		i++
	}
}

func showVerboseInfo(wallet account.Client) {
	accounts := wallet.GetAccounts()
	coins := wallet.GetCoins()

	for _, account := range accounts {
		programHash := account.ProgramHash
		assets := make(map[Uint256]Fixed64)
		address, _ := programHash.ToAddress()
		for _, out := range coins {
			if out.Output.ProgramHash == programHash {
				if _, ok := assets[out.Output.AssetID]; !ok {
					assets[out.Output.AssetID] = out.Output.Value
				} else {
					assets[out.Output.AssetID] += out.Output.Value
				}
			}
		}
		fmt.Println("---------------------------------------------------------------------------------------------------")
		fmt.Printf("Address: %s  ProgramHash: %s\n", address, BytesToHexString(programHash.ToArrayReverse()))
		if len(assets) == 0 {
			continue
		}

		fmt.Println(" ID   Asset ID\t\t\t\t\t\t\t\tAmount")
		fmt.Println("----  --------\t\t\t\t\t\t\t\t------")
		i := 0
		for id, amount := range assets {
			fmt.Printf("%4s  %s  %v\n", strconv.Itoa(i), BytesToHexString(id.ToArrayReverse()), amount)
			i++
		}
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
	passwd := c.String("password")

	// create wallet
	if c.Bool("create") {
		if FileExisted(name) {
			fmt.Printf("CAUTION: '%s' already exists!\n", name)
			os.Exit(1)
		} else {
			wallet, err := account.Create(name, getConfirmedPassword(passwd))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			showAccountsInfo(wallet)
		}
		return nil
	}

	// list wallet info
	if item := c.String("list"); item != "" {
		if item != "account" && item != "balance" && item != "verbose" {
			fmt.Fprintln(os.Stderr, "--list [account | balance | verbose]")
			os.Exit(1)
		} else {
			wallet, err := account.Open(name, getPassword(passwd))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			switch item {
			case "account":
				showAccountsInfo(wallet)
			case "balance":
				showBalancesInfo(wallet)
			case "verbose":
				showVerboseInfo(wallet)
			}
		}
		return nil
	}

	// change password
	if c.Bool("changepassword") {
		fmt.Printf("Wallet File: '%s'\n", name)
		passwd, _ := password.GetPassword()
		wallet, err := account.Open(name, passwd)
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

	// rebuild index
	if c.Bool("reset") {
		wallet, err := account.Open(name, getPassword(passwd))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := wallet.Rebuild(); err != nil {
			fmt.Fprintln(os.Stderr, "delete coins info from wallet file error")
			os.Exit(1)
		}
		fmt.Printf("%s was reset successfully\n", name)

		return nil
	}

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "wallet",
		Usage:       "user wallet operation",
		Description: "With nodectl wallet, you could control your asset.",
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
				Value: account.WalletFileName,
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
