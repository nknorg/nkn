package name

import (
	"encoding/hex"
	"fmt"
	"os"

	api "github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/httpjson/client"
	nknc "github.com/nknorg/nkn/v2/cmd/nknc/common"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/vault"

	"github.com/urfave/cli"
)

func nameAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	walletName := c.String("wallet")
	passwd := c.String("password")
	myWallet, err := vault.OpenWallet(walletName, nknc.GetPassword(passwd))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var txnFee common.Fixed64
	fee := c.String("fee")
	if fee == "" {
		txnFee = common.Fixed64(0)
	} else {
		txnFee, _ = common.StringToFixed64(fee)
	}

	regFeeString := c.String("regfee")
	var regFee common.Fixed64
	if regFeeString == "" {
		regFee = common.Fixed64(0)
	} else {
		regFee, _ = common.StringToFixed64(regFeeString)
	}

	nonce := c.Uint64("nonce")

	var resp []byte
	switch {
	case c.Bool("reg"):
		name := c.String("name")
		if name == "" {
			fmt.Println("name is required with [--name]")
			return nil
		}
		txn, _ := api.MakeRegisterNameTransaction(myWallet, name, nonce, regFee, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(nknc.Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case c.Bool("transfer"):
		name := c.String("name")
		if name == "" {
			fmt.Println("name is required with [--name]")
			return nil
		}
		to := c.String("to")
		if to == "" {
			fmt.Println("transfer is required with [--to]")
			return nil
		}
		var toBytes []byte
		toBytes, err = hex.DecodeString(to)
		if err != nil {
			return err
		}
		txn, _ := api.MakeTransferNameTransaction(myWallet, name, nonce, txnFee, toBytes)
		buff, _ := txn.Marshal()
		resp, err = client.Call(nknc.Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case c.Bool("del"):
		name := c.String("name")
		if name == "" {
			fmt.Println("name is required with [--name]")
			return nil
		}

		txn, _ := api.MakeDeleteNameTransaction(myWallet, name, nonce, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(nknc.Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case c.Bool("get"):
		name := c.String("name")
		if name == "" {
			fmt.Println("name is required with [--name]")
			return nil
		}
		resp, err = client.Call(nknc.Address(), "getregistrant", 0, map[string]interface{}{"name": name})
	default:
		cli.ShowSubcommandHelp(c)
		return nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	nknc.FormatOutput(resp)

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "name",
		Usage:       "name registration",
		Description: "With nknc name, you could register name for your address.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "reg, r",
				Usage: "register name for your address",
			},
			cli.BoolFlag{
				Name:  "del, d",
				Usage: "delete name of your address",
			},
			cli.BoolFlag{
				Name:  "get, g",
				Usage: "get register name info",
			},
			cli.BoolFlag{
				Name:  "transfer, t",
				Usage: "transfer name to another address",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "name",
			},
			cli.StringFlag{
				Name:  "wallet, w",
				Usage: "wallet name",
				Value: config.Parameters.WalletFile,
			},
			cli.StringFlag{
				Name:  "password, p",
				Usage: "wallet password",
			},
			cli.StringFlag{
				Name:  "fee, f",
				Usage: "transaction fee",
				Value: "",
			},
			cli.Uint64Flag{
				Name:  "nonce",
				Usage: "nonce",
			},
			cli.StringFlag{
				Name:  "regfee",
				Usage: "regfee",
			},
			cli.StringFlag{
				Name:  "to",
				Usage: "transfer name to addr",
			},
		},
		Action: nameAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			nknc.PrintError(c, err, "name")
			return cli.NewExitError("", 1)
		},
	}
}
