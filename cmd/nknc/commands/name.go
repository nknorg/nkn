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

// nameCmd represents the name command
var nameCmd = &cobra.Command{
	Use:   "name",
	Short: "name registration",
	Long:  "With nknc name, you could register name for your address.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nameAction(cmd)
	},
}

var (
	reg    bool
	del    bool
	get    bool
	name   string
	regfee string
)

func init() {
	rootCmd.AddCommand(nameCmd)

	nameCmd.Flags().BoolVarP(&reg, "reg", "r", false, "register name for your address")
	nameCmd.Flags().BoolVarP(&del, "del", "d", false, "delete name of your address")
	nameCmd.Flags().BoolVarP(&get, "get", "g", false, "get register name info")
	nameCmd.Flags().BoolVarP(&transfer, "transfer", "t", false, "transfer name to another address")
	nameCmd.Flags().StringVar(&name, "name", "", "name")
	nameCmd.Flags().StringVarP(&walletFile, "wallet", "w", config.Parameters.WalletFile, "wallet name")
	nameCmd.Flags().StringVar(&walletPassword, "password", "", "wallet password")
	nameCmd.Flags().StringVarP(&fee, "fee", "f", "", "transaction fee")
	nameCmd.Flags().Uint64Var(&nonce, "nonce", 0, "nonce")
	nameCmd.Flags().StringVar(&regfee, "regfee", "", "regfee")
	nameCmd.Flags().StringVar(&to, "to", "", "transfer name to addr")

	nameCmd.MarkFlagRequired("password")
}

func nameAction(cmd *cobra.Command) error {
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
			return err
		}
	}

	var regFee common.Fixed64
	if regfee == "" {
		regFee = common.Fixed64(0)
	} else {
		regFee, _ = common.StringToFixed64(regfee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var resp []byte
	switch {
	case reg:
		if name == "" {
			return fmt.Errorf("name is required with [--name]")
		}
		txn, _ := api.MakeRegisterNameTransaction(myWallet, name, nonce, regFee, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case transfer:
		if name == "" {
			return fmt.Errorf("name is required with [--name]")
		}
		if to == "" {
			return fmt.Errorf("transfer is required with [--to]")
		}
		var toBytes []byte
		toBytes, err = hex.DecodeString(to)
		if err != nil {
			return err
		}
		txn, _ := api.MakeTransferNameTransaction(myWallet, name, nonce, txnFee, toBytes)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case del:
		if name == "" {
			return fmt.Errorf("name is required with [--name]")
		}

		txn, _ := api.MakeDeleteNameTransaction(myWallet, name, nonce, txnFee)
		buff, _ := txn.Marshal()
		resp, err = client.Call(Address(), "sendrawtransaction", 0, map[string]interface{}{"tx": hex.EncodeToString(buff)})
	case get:
		if name == "" {
			return fmt.Errorf("name is required with [--name]")
		}
		resp, err = client.Call(Address(), "getregistrant", 0, map[string]interface{}{"name": name})
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
