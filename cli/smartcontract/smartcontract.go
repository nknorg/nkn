package smartcontract

import (
	"DNA/account"
	. "DNA/cli/common"
	"DNA/common"
	"DNA/common/password"
	"DNA/core/code"
	"DNA/core/contract"
	"DNA/core/signature"
	"DNA/core/transaction"
	httpjsonrpc "DNA/net/httpjsonrpc"
	"DNA/smartcontract/types"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
)

func newContractContextWithoutProgramHashes(data signature.SignableData) *contract.ContractContext {
	return &contract.ContractContext{
		Data:       data,
		Codes:      make([][]byte, 1),
		Parameters: make([][][]byte, 1),
	}
}

func openWallet(name string, passwd string) account.Client {
	if name == "" {
		name = account.WalletFileName
		fmt.Println("Using default wallet: ", account.WalletFileName)
	}
	pwd := []byte(passwd)
	var err error
	if passwd == "" {
		pwd, err = password.GetPassword()
		if err != nil {
			fmt.Println("Get password error.")
			os.Exit(1)
		}
	}
	wallet := account.Open(name, pwd)
	if wallet == nil {
		fmt.Println("Failed to open wallet: ", name)
		os.Exit(1)
	}
	return wallet
}
func signTransaction(signer *account.Account, tx *transaction.Transaction) error {
	signature, err := signature.SignBySigner(tx, signer)
	if err != nil {
		fmt.Println("SignBySigner failed")
		return err
	}
	transactionContract, err := contract.CreateSignatureContract(signer.PubKey())
	if err != nil {
		fmt.Println("CreateSignatureContract failed")
		return err
	}
	transactionContractContext := newContractContextWithoutProgramHashes(tx)
	if err := transactionContractContext.AddContract(transactionContract, signer.PubKey(), signature); err != nil {
		fmt.Println("AddContract failed")
		return err
	}
	tx.SetPrograms(transactionContractContext.GetPrograms())
	return nil
}
func makeDeployContractTransaction(signer *account.Account, codeStr string, language int) (string, error) {
	c, _ := common.HexToBytes(codeStr)
	fc := &code.FunctionCode{
		Code:           c,
		ParameterTypes: []contract.ContractParameterType{contract.ByteArray, contract.ByteArray},
		ReturnType:     contract.ContractParameterType(contract.Object),
	}
	fc.CodeHash()

	tx, err := transaction.NewDeployTransaction(fc, signer.ProgramHash, "DNA", "1.0", "DNA user", "user@onchain.com", "test uint", types.LangType(byte(language)))
	if err != nil {
		return "Deploy smartcontract fail!", err
	}
	txAttr := transaction.NewTxAttribute(transaction.Nonce, []byte(strconv.FormatInt(rand.Int63(), 10)))
	tx.Attributes = make([]*transaction.TxAttribute, 0)
	tx.Attributes = append(tx.Attributes, &txAttr)

	var buffer bytes.Buffer
	if err := tx.Serialize(&buffer); err != nil {
		fmt.Println("serialize registtransaction failed")
		return "", err
	}
	return hex.EncodeToString(buffer.Bytes()), nil
}

func makeInvokeTransaction(signer *account.Account, paramsStr, codeHashStr string) (string, error) {
	p, _ := common.HexToBytes(paramsStr)
	hash, _ := common.HexToBytesReverse(codeHashStr)
	p = append(p, 0x69)
	p = append(p, hash...)
	codeHash := common.BytesToUint160(hash)
	transactionContract, err := contract.CreateSignatureContract(signer.PubKey())
	if err != nil {
		fmt.Println("CreateSignatureContract failed")
		return "", err
	}

	tx, err := transaction.NewInvokeTransaction(p, codeHash, transactionContract.ProgramHash)
	if err != nil {
		return "Invoke smartcontract fail!", err
	}
	txAttr := transaction.NewTxAttribute(transaction.Nonce, []byte(strconv.FormatInt(rand.Int63(), 10)))
	tx.Attributes = make([]*transaction.TxAttribute, 0)
	tx.Attributes = append(tx.Attributes, &txAttr)

	if err := signTransaction(signer, tx); err != nil {
		fmt.Println("sign transfer transaction failed")
		return "", err
	}
	var buffer bytes.Buffer
	if err := tx.Serialize(&buffer); err != nil {
		fmt.Println("serialize registtransaction failed")
		return "", err
	}
	return hex.EncodeToString(buffer.Bytes()), nil
}

func contractAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		cli.ShowSubcommandHelp(c)
		return nil
	}
	var err error
	var txHex string
	deploy := c.Bool("deploy")
	invoke := c.Bool("invoke")

	if !deploy && !invoke {
		fmt.Println("missing --deploy -d or --invoke -i")
		return nil
	}

	wallet := openWallet(c.String("wallet"), c.String("password"))
	admin, _ := wallet.GetDefaultAccount()

	if deploy {
		codeStr := c.String("code")
		fileStr := c.String("file")
		language := c.Int("language")
		if codeStr == "" && fileStr == "" {
			fmt.Println("missing args [--code] or [--file]")
			return nil
		}
		if codeStr != "" && fileStr != "" {
			fmt.Println("too many input args")
			return nil
		}
		if fileStr != "" {
			bytes, err := ioutil.ReadFile(fileStr)
			if err != nil {
				fmt.Println("read avm file err")
				return nil
			}
			codeStr = common.ToHexString(bytes)
		}
		txHex, err = makeDeployContractTransaction(admin, codeStr, language)
		if err != nil {
			fmt.Println(err)
		}
	}
	if invoke {
		paramsStr := c.String("params")
		codeHashStr := c.String("codeHash")
		if codeHashStr == "" {
			fmt.Println("missing args [--codeHash]")
			return nil
		}
		if paramsStr == "" {
			fmt.Println("missing args [--params]")
			return nil
		}
		txHex, err = makeInvokeTransaction(admin, paramsStr, codeHashStr)

		if err != nil {
			fmt.Println(err)
		}
	}
	resp, err := httpjsonrpc.Call(Address(), "sendrawtransaction", 0, []interface{}{txHex})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	FormatOutput(resp)
	return nil
}

func NewCommand() *cli.Command {
	return &cli.Command{
		Name:        "contract",
		Usage:       "deploy or invoke your smartcontract ",
		Description: "you could deploy or invoke your smartcontract.",
		ArgsUsage:   "[args]",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "deploy, d",
				Usage: "deploy smartcontract",
			},
			cli.BoolFlag{
				Name:  "invoke, i",
				Usage: "invoke smartcontract",
			},
			cli.StringFlag{
				Name:  "code, c",
				Usage: "deploy contract code",
			},
			cli.StringFlag{
				Name:  "file, f",
				Usage: "deploy avm file",
			},
			cli.IntFlag{
				Name:  "language, l",
				Usage: "deploy contract compiler contract language",
			},
			cli.StringFlag{
				Name:  "params, p",
				Usage: "invoke contract compiler contract params",
			},
			cli.StringFlag{
				Name:  "codeHash, a",
				Usage: "invoke contract compiler contract code hash",
			},
		},
		Action: contractAction,
		OnUsageError: func(c *cli.Context, err error, isSubcommand bool) error {
			PrintError(c, err, "smartcontract")
			return cli.NewExitError("", 1)
		},
	}
}
