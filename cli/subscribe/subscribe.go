package subscribe

import (
	"encoding/hex"
	"fmt"
	"os"

	. "github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/httpjson/client"
	. "github.com/nknorg/nkn/cli/common"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vault"

	"github.com/urfave/cli"
)

func subscribeAction(c *cli.Context) error {
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

	nonce := c.Uint64("nonce")

	var resp []byte
	switch {
	case c.Bool("sub"):
		id := c.String("identifier")
		if id == "" {
			fmt.Println("identifier is required with [--id]")
			return nil
		}

		topic := c.String("topic")
		if topic == "" {
			fmt.Println("topic is required with [--topic]")
			return nil
		}

		bucket := c.Uint64("bucket")
		duration := c.Uint64("duration")

		meta := c.String("meta")
		if meta == "" {
			fmt.Println("meta is required with [--meta]")
			return nil
		}

		txn, _ := MakeSubscribeTransaction(myWallet, id, topic, uint32(bucket), uint32(duration), meta, nonce, txnFee)
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
		Name:        "subscribe",
		Usage:       "subscribe topic",
		Description: "With nknc sub, you could subscribe your topic.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "sub, s",
				Usage: "subscribe your topic",
			},
			cli.StringFlag{
				Name:  "identifier, id",
				Usage: "identifier",
			},
			cli.StringFlag{
				Name:  "topic",
				Usage: "topic",
			},
			cli.Uint64Flag{
				Name:  "bucket",
				Usage: "bucket",
			},
			cli.Uint64Flag{
				Name:  "duration",
				Usage: "duration",
			},
			cli.StringFlag{
				Name:  "meta",
				Usage: "meta",
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
		},
		Action: subscribeAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "subscribe")
			return cli.NewExitError("", 1)
		},
	}
}
