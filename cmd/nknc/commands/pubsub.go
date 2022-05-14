package commands

import (
	"encoding/hex"
	"fmt"
	"os"

	api "github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/httpjson/client"
	"github.com/nknorg/nkn/v2/common"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/spf13/cobra"
)

// pubsubCmd represents the pubsub command
var pubsubCmd = &cobra.Command{
	Use:   "pubsub",
	Short: "manage topic subscriptions",
	Long:  "With nknc pubsub, you could manage topic subscriptions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return subscribeAction(cmd)
	},
}

var (
	sub        bool
	unsub      bool
	identifier string
	topic      string
	duration   uint64
	offset     uint64
	limit      uint64
	subscriber string
	meta       string
)

func init() {
	rootCmd.AddCommand(pubsubCmd)

	pubsubCmd.Flags().BoolVarP(&sub, "sub", "s", false, "subscribe to topic")
	pubsubCmd.Flags().BoolVarP(&unsub, "unsub", "u", false, "unsubscribe from topic")
	pubsubCmd.Flags().BoolVarP(&get, "get", "g", false, "get subscribes list of specified topic or get details info of a specified subscriber")
	pubsubCmd.Flags().StringVar(&identifier, "id", "", "identifier")
	pubsubCmd.Flags().StringVar(&topic, "topic", "", "topic")
	pubsubCmd.Flags().Uint64Var(&duration, "duration", 0, "duration")
	pubsubCmd.Flags().Uint64Var(&offset, "offset", 0, "get subscribes skip previous offset items")
	pubsubCmd.Flags().Uint64Var(&limit, "limit", 0, "limit subscribes amount which start from offset")
	pubsubCmd.Flags().StringVar(&subscriber, "subscriber", "", "specified subscriber which you want details")
	pubsubCmd.Flags().StringVar(&meta, "meta", "", "meta")
	pubsubCmd.Flags().StringVarP(&walletFile, "wallet", "w", config.Parameters.WalletFile, "wallet name")
	pubsubCmd.Flags().StringVarP(&walletPassword, "password", "p", "", "wallet password")
	pubsubCmd.Flags().StringVarP(&fee, "fee", "f", "", "transaction fee")
	pubsubCmd.Flags().Uint64Var(&nonce, "nonce", 0, "nonce")

}

func subscribeAction(cmd *cobra.Command) error {
	myWallet, err := vault.OpenWallet(walletFile, GetPassword(walletPassword))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var txnFee common.Fixed64
	if fee == "" {
		txnFee = common.Fixed64(0)
	} else {
		txnFee, _ = common.StringToFixed64(fee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var resp []byte
	switch {
	case sub:
		if identifier == "" {
			return fmt.Errorf("identifier is required with [--id]")
		}

		if topic == "" {
			return fmt.Errorf("topic is required with [--topic]")
		}

		if meta == "" {
			return fmt.Errorf("meta is required with [--meta]")
		}

		txn, _ := api.MakeSubscribeTransaction(myWallet, identifier, topic, uint32(duration), meta, nonce, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case unsub:
		id := identifier
		if id == "" {
			return fmt.Errorf("identifier is required with [--id]")
		}

		if topic == "" {
			return fmt.Errorf("topic is required with [--topic]")
		}

		txn, _ := api.MakeUnsubscribeTransaction(myWallet, id, topic, nonce, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case get:
		if topic == "" {
			return fmt.Errorf("topic is required with [--topic]")
		}

		params := map[string]interface{}{"topic": topic}
		if subscriber == "" { // get list of topic
			params["offset"] = offset
			params["limit"] = limit
			resp, err = client.Call(Address(), "getsubscribers", 0, params)
		} else { // get details of specified subscriber
			params["subscriber"] = subscriber
			resp, err = client.Call(Address(), "getsubscription", 0, params)
		}
	default:
		return cmd.Usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	FormatOutput(resp)

	return nil
}
