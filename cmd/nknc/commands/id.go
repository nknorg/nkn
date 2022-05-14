package commands

import (
	"context"
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

// idCmd represents the id command
var idCmd = &cobra.Command{
	Use:   "id",
	Short: "generate id for nknd",
	Long:  "With nknc id, you could generate ID.",
	Run: func(cmd *cobra.Command, args []string) {
		generateIDAction(cmd)
	},
}

var (
	genid     bool
	pubkeyHex string
)

func init() {
	rootCmd.AddCommand(idCmd)

	idCmd.Flags().BoolVar(&genid, "genid", false, "generate id")
	idCmd.Flags().StringVar(&pubkeyHex, "pubkey", "", "pubkey to generate id for, leave empty for local wallet pubkey")
	idCmd.Flags().StringVarP(&walletFile, "wallet", "w", config.Parameters.WalletFile, "wallet name")
	idCmd.Flags().StringVarP(&walletPassword, "password", "p", "", "wallet password")
	idCmd.Flags().StringVar(&regfee, "regfee", "", "registration fee")
	idCmd.Flags().StringVarP(&fee, "fee", "f", "", "transaction fee")
	idCmd.Flags().Uint64Var(&nonce, "nonce", 0, "nonce")
}

func generateIDAction(cmd *cobra.Command) error {
	pubkey, err := hex.DecodeString(pubkeyHex)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	myWallet, err := vault.OpenWallet(walletFile, GetPassword(walletPassword))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var txnFee common.Fixed64
	if regfee == "" {
		txnFee = common.Fixed64(0)
	} else {
		txnFee, err = common.StringToFixed64(regfee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var regFee common.Fixed64
	fee = regfee
	if fee == "" {
		regFee = common.Fixed64(0)
	} else {
		regFee, err = common.StringToFixed64(fee)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var resp []byte
	switch {
	case genid:
		account, err := myWallet.GetDefaultAccount()
		if err != nil {
			return err
		}

		walletAddr, err := account.ProgramHash.ToAddress()
		if err != nil {
			return err
		}

		remoteNonce, height, err := client.GetNonceByAddr(Address(), walletAddr, true)
		if err != nil {
			return err
		}

		if nonce == 0 {
			nonce = remoteNonce
		}

		txn, err := api.MakeGenerateIDTransaction(context.Background(), pubkey, myWallet, regFee, nonce, txnFee, height+1)
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
