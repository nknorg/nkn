package id

import (
	"encoding/hex"
	"fmt"
	"os"

	. "github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/vault"

	"github.com/urfave/cli"
)

func generateIDAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	walletName := c.String("wallet")
	passwd := c.String("password")
	myWallet, err := vault.OpenWallet(walletName, GetPassword(passwd))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var txnFee Fixed64
	fee := c.String("fee")
	if fee == "" {
		txnFee = Fixed64(0)
	} else {
		txnFee, _ = StringToFixed64(fee)
	}

	var regFee Fixed64
	fee = c.String("regfee")
	if fee == "" {
		regFee = Fixed64(0)
	} else {
		regFee, _ = StringToFixed64(fee)
	}

	nonce := c.Uint64("nonce")

	var resp []byte
	switch {
	case c.Bool("genid"):
		txn, _ := MakeGenerateIDTransaction(myWallet, regFee, nonce, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	default:
		cli.ShowSubcommandHelp(c)
		return nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	FormatOutput(resp)

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "id",
		Usage:       "generate id for nknd",
		Description: "With nknc id, you could generate ID.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "genid",
				Usage: "generate id",
			},
			cli.StringFlag{
				Name:  "wallet, w",
				Usage: "wallet name",
				Value: vault.WalletFileName,
			},
			cli.StringFlag{
				Name:  "password, p",
				Usage: "wallet password",
			},
			cli.StringFlag{
				Name:  "regfee",
				Usage: "registration fee",
				Value: "",
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
		},
		Action: generateIDAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "id")
			return cli.NewExitError("", 1)
		},
	}
}
