package asset

import (
	"encoding/hex"
	"fmt"
	"os"

	. "github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"

	"github.com/urfave/cli"
)

const (
	RANDBYTELEN = 4
)

func parseAddress(c *cli.Context) Uint160 {
	if address := c.String("to"); address != "" {
		pg, err := ToScriptHash(address)
		if err != nil {
			fmt.Println("invalid receiver address")
			os.Exit(1)
		}
		return pg
	}
	fmt.Println("missing flag [--to]")
	os.Exit(1)
	return EmptyUint160
}

func assetAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	value := c.String("value")
	if value == "" {
		fmt.Println("asset amount is required with [--value]")
		return nil
	}

	var txnFee Fixed64
	fee := c.String("fee")
	var err error
	if fee == "" {
		txnFee = Fixed64(0)
	} else {
		txnFee, err = StringToFixed64(fee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	}

	nonce := c.Uint64("nonce")

	var resp []byte
	switch {
	case c.Bool("issue"):
		walletName := c.String("wallet")
		passwd := c.String("password")
		myWallet, err := vault.OpenWallet(walletName, getPassword(passwd))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		name := c.String("name")
		if name == "" {
			fmt.Println("asset name is required with [--name]")
			return nil
		}

		symbol := c.String("symbol")
		if name == "" {
			fmt.Println("asset symbol is required with [--symbol]")
			return nil
		}

		totalSupply, err := StringToFixed64(value)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		precision := uint32(c.Uint("precision"))
		if precision > config.MaxAssetPrecision {
			err := fmt.Errorf("precision is larger than %v", config.MaxAssetPrecision)
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		txn, err := MakeIssueAssetTransaction(myWallet, name, symbol, totalSupply, precision, nonce, txnFee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		buff, err := txn.Marshal()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	case c.Bool("transfer"):
		walletName := c.String("wallet")
		passwd := c.String("password")
		myWallet, err := vault.OpenWallet(walletName, getPassword(passwd))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		receipt := parseAddress(c)
		amount, err := StringToFixed64(value)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		txn, err := MakeTransferTransaction(myWallet, receipt, nonce, amount, txnFee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		buff, err := txn.Marshal()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	default:
		cli.ShowSubcommandHelp(c)
		return nil
	}

	FormatOutput(resp)

	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "asset",
		Usage:       "asset registration, issuance and transfer",
		Description: "With nknc asset, you could control assert through transaction.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "issue, i",
				Usage: "issue asset",
			},
			cli.BoolFlag{
				Name:  "transfer, t",
				Usage: "transfer asset",
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
				Name:  "to",
				Usage: "asset to whom",
			},
			cli.StringFlag{
				Name:  "value, v",
				Usage: "asset amount in transfer asset or totalSupply in inssue assset",
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
			cli.StringFlag{
				Name:  "name",
				Usage: "asset name",
			},
			cli.StringFlag{
				Name:  "symbol",
				Usage: "asset symbol",
			},
			cli.UintFlag{
				Name:  "precision",
				Usage: "asset precision",
			},
		},
		Action: assetAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "asset")
			return cli.NewExitError("", 1)
		},
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
