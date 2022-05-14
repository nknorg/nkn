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

const (
	RANDBYTELEN = 4
)

// assetCmd represents the asset command
var assetCmd = &cobra.Command{
	Use:   "asset",
	Short: "asset registration, issuance and transfer",
	Long:  "",
	RunE: func(cmd *cobra.Command, args []string) error {
		return assetAction(cmd)
	},
}

var (
	issue     bool
	transfer  bool
	to        string
	value     string
	assetname string
	symbol    string
	precision uint32
)

func init() {
	rootCmd.AddCommand(assetCmd)

	assetCmd.Flags().BoolVarP(&issue, "issue", "i", false, "issue asset")
	assetCmd.Flags().BoolVarP(&transfer, "transfer", "t", false, "transfer asset")
	assetCmd.Flags().StringVarP(&walletFile, "wallet", "w", config.Parameters.WalletFile, "wallet name")
	assetCmd.Flags().StringVar(&walletPassword, "password", "", "wallet password")
	assetCmd.Flags().StringVar(&to, "to", "", "asset to whom")
	assetCmd.Flags().StringVarP(&value, "value", "v", "", "asset amount in transfer asset or totalSupply in inssue assset")
	assetCmd.Flags().StringVarP(&fee, "fee", "f", "", "transaction fee")
	assetCmd.Flags().Uint64Var(&nonce, "nonce", 0, "nonce")
	assetCmd.Flags().StringVar(&assetname, "name", "", "asset name")
	assetCmd.Flags().StringVar(&symbol, "symbol", "", "asset symbol")
	assetCmd.Flags().Uint32Var(&precision, "precision", 0, "asset precision")

	assetCmd.Flags().MarkHidden("password")
}

func parseAddress() common.Uint160 {
	if address := to; address != "" {
		pg, err := common.ToScriptHash(address)
		if err != nil {
			fmt.Println("invalid receiver address")
			os.Exit(1)
		}
		return pg
	}
	fmt.Println("missing flag [--to]")
	os.Exit(1)
	return common.EmptyUint160
}

func assetAction(cmd *cobra.Command) error {
	var err error
	var txnFee common.Fixed64

	if fee == "" {
		txnFee = common.Fixed64(0)
	} else {
		txnFee, err = common.StringToFixed64(fee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
	}

	var resp []byte
	switch {
	case issue:
		myWallet, err := vault.OpenWallet(walletFile, GetPassword(walletPassword))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if assetname == "" {
			return fmt.Errorf("asset name is required with [--name]")
		}

		if symbol == "" {
			return fmt.Errorf("asset symbol is required with [--symbol]")
		}

		totalSupply, err := common.StringToFixed64(value)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		if precision > config.MaxAssetPrecision {
			err := fmt.Errorf("precision is larger than %v", config.MaxAssetPrecision)
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		txn, err := api.MakeIssueAssetTransaction(myWallet, assetname, symbol, totalSupply, precision, nonce, txnFee)
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
	case transfer:
		myWallet, err := vault.OpenWallet(walletFile, GetPassword(walletPassword))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		receipt := parseAddress()
		amount, err := common.StringToFixed64(value)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}

		if nonce == 0 {
			remoteNonce, _, err := client.GetNonceByAddr(Address(), myWallet.Address, true)
			if err != nil {
				return err
			}
			nonce = remoteNonce
		}

		txn, err := api.MakeTransferTransaction(myWallet, receipt, nonce, amount, txnFee)
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
		return cmd.Usage()
	}

	FormatOutput(resp)

	return nil
}
