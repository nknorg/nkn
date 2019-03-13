package asset

import (
	"encoding/hex"
	"fmt"
	"os"

	. "github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto/util"
	"github.com/nknorg/nkn/util/password"
	"github.com/nknorg/nkn/vault"

	"github.com/urfave/cli"
)

const (
	RANDBYTELEN = 4
)

func parseAssetName(c *cli.Context) string {
	name := c.String("name")
	if name == "" {
		name = "TEST-" + BytesToHexString(util.RandomBytes(RANDBYTELEN))
	}

	return name
}

func parseAssetID(c *cli.Context) string {
	asset := c.String("asset")
	if asset == "" {
		fmt.Println("missing flag [--asset]")
		os.Exit(1)
	}

	return asset
}

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

	var err error
	var resp []byte
	switch {
	case c.Bool("reg"):
		resp, err = client.Call(Address(), "registasset", 0, map[string]interface{}{"name": parseAssetName(c), "value": value})
	case c.Bool("issue"):
		resp, err = client.Call(Address(), "issueasset", 0, map[string]interface{}{"assetid": parseAssetID(c), "address": parseAddress(c), "value": value})
	case c.Bool("transfer"):
		walletName := c.String("wallet")
		passwd := c.String("password")
		myWallet, err := vault.OpenWallet(walletName, getPassword(passwd))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		receipt := parseAddress(c)
		amount, _ := StringToFixed64(value)

		var txnFee Fixed64
		fee := c.String("fee")
		if fee == "" {
			txnFee = Fixed64(0)
		} else {
			txnFee, _ = StringToFixed64(fee)
		}

		nonce := c.Uint64("nonce")
		txn, _ := MakeTransferTransaction(myWallet, receipt, nonce, amount, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case c.Bool("prepaid"):
		rates := c.String("rates")
		if rates == "" {
			fmt.Println("rates is required with [--rates]")
			return nil
		}
		resp, err = client.Call(Address(), "prepaidasset", 0, map[string]interface{}{"assetid": parseAssetID(c), "value": value, "rates": rates})
	case c.Bool("withdraw"):
		resp, err = client.Call(Address(), "withdrawasset", 0, map[string]interface{}{"assetid": parseAssetID(c), "value": value})
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
		Name:        "asset",
		Usage:       "asset registration, issuance and transfer",
		Description: "With nknc asset, you could control assert through transaction.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "reg, r",
				Usage: "regist a new kind of asset",
			},
			cli.BoolFlag{
				Name:  "issue, i",
				Usage: "issue asset that has been registered",
			},
			cli.BoolFlag{
				Name:  "transfer, t",
				Usage: "transfer asset",
			},
			cli.BoolFlag{
				Name:  "prepaid",
				Usage: "prepaid asset",
			},
			cli.BoolFlag{
				Name:  "withdraw",
				Usage: "withdraw asset",
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
				Name:  "asset, a",
				Usage: "uniq id for asset",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "asset name",
			},
			cli.StringFlag{
				Name:  "to",
				Usage: "asset to whom",
			},
			cli.StringFlag{
				Name:  "value, v",
				Usage: "asset amount",
				Value: "",
			},
			cli.StringFlag{
				Name:  "fee, f",
				Usage: "transaction fee",
				Value: "",
			},
			cli.StringFlag{
				Name:  "rates",
				Usage: "rates",
			},
			cli.Uint64Flag{
				Name:  "nonce",
				Usage: "nonce",
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
