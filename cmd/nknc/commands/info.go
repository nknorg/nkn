package commands

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nknorg/nkn/v2/api/httpjson/client"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/proto"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "show blockchain information",
	Long:  "",
	RunE: func(cmd *cobra.Command, args []string) error {
		showusage := true
		cmd.Flags().Visit(func(name *pflag.Flag) {
			showusage = false
		})
		if showusage {
			return cmd.Usage()
		}
		if err := infoAction(); err != nil {
			log.Error(err)
		}
		return nil
	},
}

var (
	pretty          bool
	blockhash       string
	txhash          string
	latestblockhash bool
	height          int
	header          int
	headerhash      string
	blockcount      bool
	connections     bool
	neighbor        bool
	ring            bool
	state           bool
	nodeversion     bool
	balance         string
	idstr           string
)

func init() {
	rootCmd.AddCommand(infoCmd)

	infoCmd.Flags().BoolVarP(&pretty, "pretty", "p", false, "pretty print")
	infoCmd.Flags().StringVarP(&blockhash, "blockhash", "b", "", "hash for querying a block")
	infoCmd.Flags().StringVarP(&txhash, "txhash", "t", "", "hash for querying a transaction")
	infoCmd.Flags().BoolVar(&latestblockhash, "latestblockhash", false, "latest block hash")
	infoCmd.Flags().IntVar(&height, "height", -1, "block height for querying a block")
	infoCmd.Flags().IntVar(&header, "header", -1, "get block header by height")
	infoCmd.Flags().StringVar(&headerhash, "headerhash", "", "get block header by hash")
	infoCmd.Flags().BoolVarP(&blockcount, "blockcount", "c", false, "block number in blockchain")
	infoCmd.Flags().BoolVar(&connections, "connections", false, "connection count")
	infoCmd.Flags().BoolVar(&neighbor, "neighbor", false, "neighbor information of current node")
	infoCmd.Flags().BoolVar(&ring, "ring", false, "chord ring information of current node")
	infoCmd.Flags().BoolVarP(&state, "state", "s", false, "current node state")
	infoCmd.Flags().BoolVarP(&nodeversion, "nodeversion", "v", false, "version of connected remote node")
	infoCmd.Flags().StringVar(&balance, "balance", "", "balance of a address")
	infoCmd.Flags().Uint64Var(&nonce, "nonce", 0, "nonce of a address")
	infoCmd.Flags().StringVar(&idstr, "id", "", "id from publickey")

}

func infoAction() (err error) {
	var resp []byte
	var output [][]byte

	version := nodeversion
	id := idstr

	if height >= 0 {
		resp, err = client.Call(Address(), "getblock", 0, map[string]interface{}{"height": height})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		if pretty {
			if p, err := PrettyPrinter(resp).PrettyBlock(); err == nil {
				resp = p // replace resp if pretty success
			} else {
				fmt.Fprintln(os.Stderr, "Fallback to original resp due to PrettyPrint fail: ", err)
			}
		}
		output = append(output, resp)
	}

	if len(blockhash) > 0 {
		resp, err = client.Call(Address(), "getblock", 0, map[string]interface{}{"hash": blockhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		if pretty {
			if p, err := PrettyPrinter(resp).PrettyBlock(); err == nil {
				resp = p // replace resp if pretty success
			} else {
				fmt.Fprintln(os.Stderr, "Fallback to original resp due to PrettyPrint fail: ", err)
			}
		}
		output = append(output, resp)
	}

	if header >= 0 {
		resp, err = client.Call(Address(), "getheader", 0, map[string]interface{}{"height": header})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if len(headerhash) > 0 {
		resp, err = client.Call(Address(), "getheader", 0, map[string]interface{}{"hash": headerhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if latestblockhash {
		resp, err = client.Call(Address(), "getlatestblockhash", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if blockcount {
		resp, err = client.Call(Address(), "getblockcount", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if connections {
		resp, err = client.Call(Address(), "getconnectioncount", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if neighbor {
		resp, err := client.Call(Address(), "getneighbor", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if ring {
		resp, err := client.Call(Address(), "getchordringinfo", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if state {
		resp, err := client.Call(Address(), "getnodestate", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if txhash != "" {
		resp, err = client.Call(Address(), "gettransaction", 0, map[string]interface{}{"hash": txhash})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		if pretty {
			if p, err := PrettyPrinter(resp).PrettyTxn(); err == nil {
				resp = p // replace resp if pretty success
			} else {
				fmt.Fprintln(os.Stderr, "Output origin resp due to PrettyPrint fail: ", err)
			}
		}
		output = append(output, resp)
	}

	if version {
		resp, err = client.Call(Address(), "getversion", 0, map[string]interface{}{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)

	}

	if balance != "" {
		resp, err := client.Call(Address(), "getbalancebyaddr", 0, map[string]interface{}{"address": balance})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if nonce > 0 {
		resp, err := client.Call(Address(), "getNoncebyaddr", 0, map[string]interface{}{"address": fmt.Sprintf("%d", nonce)})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	if id != "" {
		resp, err := client.Call(Address(), "getid", 0, map[string]interface{}{"publickey": id})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return err
		}
		output = append(output, resp)
	}

	for _, v := range output {
		FormatOutput(v)
	}

	return nil
}

type PrettyPrinter []byte

// TxnUnmarshal function
func TxnUnmarshal(m map[string]interface{}) (interface{}, error) {
	typ, ok := m["txType"]
	if !ok {
		return nil, fmt.Errorf("No such key [txType]")
	}

	pbHexStr, ok := m["payloadData"]
	if !ok {
		return m, nil
	}

	buf, err := hex.DecodeString(pbHexStr.(string))
	if err != nil {
		return nil, err
	}

	switch typ {
	case pb.PayloadType_name[int32(pb.PayloadType_SIG_CHAIN_TXN_TYPE)]:
		sigChainTxn := &pb.SigChainTxn{}
		if err = proto.Unmarshal(buf, sigChainTxn); err == nil { // bin to pb struct of SigChainTxnType txn
			m["payloadData"] = sigChainTxn.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_COINBASE_TYPE)]:
		coinBaseTxn := &pb.Coinbase{}
		if err = proto.Unmarshal(buf, coinBaseTxn); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = coinBaseTxn.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_TRANSFER_ASSET_TYPE)]:
		trans := &pb.TransferAsset{}
		if err = proto.Unmarshal(buf, trans); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = trans.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_GENERATE_ID_TYPE)]:
		genID := &pb.GenerateID{}
		if err = proto.Unmarshal(buf, genID); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = genID.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_REGISTER_NAME_TYPE)]:
		regName := &pb.RegisterName{}
		if err = proto.Unmarshal(buf, regName); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = regName.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_SUBSCRIBE_TYPE)]:
		sub := &pb.Subscribe{}
		if err = proto.Unmarshal(buf, sub); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = sub.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_UNSUBSCRIBE_TYPE)]:
		sub := &pb.Unsubscribe{}
		if err = proto.Unmarshal(buf, sub); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = sub.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_NANO_PAY_TYPE)]:
		pay := &pb.NanoPay{}
		if err = proto.Unmarshal(buf, pay); err == nil { // bin to pb struct of Coinbase txn
			m["payloadData"] = pay.ToMap()
		}
	case pb.PayloadType_name[int32(pb.PayloadType_TRANSFER_NAME_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.PayloadType_DELETE_NAME_TYPE)]:
		fallthrough //TODO
	case pb.PayloadType_name[int32(pb.PayloadType_ISSUE_ASSET_TYPE)]:
		fallthrough //TODO
	default:
		return nil, fmt.Errorf("Unknow txType[%s] for pretty print", typ)
	}

	return m, nil
}

func (resp PrettyPrinter) PrettyTxn() ([]byte, error) {
	m := map[string]interface{}{}
	err := json.Unmarshal(resp, &m)
	if err != nil {
		return nil, err
	}

	v, ok := m["result"]
	if !ok {
		return nil, fmt.Errorf("response No such key [result]")
	}

	if m["result"], err = TxnUnmarshal(v.(map[string]interface{})); err != nil {
		return nil, err
	}

	return json.Marshal(m)
}

func (resp PrettyPrinter) PrettyBlock() ([]byte, error) {
	m := map[string]interface{}{}
	err := json.Unmarshal(resp, &m)
	if err != nil {
		return nil, err
	}

	ret, ok := m["result"]
	if !ok {
		return nil, fmt.Errorf("response No such key [result]")
	}

	txns, ok := ret.(map[string]interface{})["transactions"]
	if !ok {
		return nil, fmt.Errorf("result No such key [transactions]")
	}

	lst := make([]interface{}, 0)
	for _, t := range txns.([]interface{}) {
		if m, err := TxnUnmarshal(t.(map[string]interface{})); err == nil {
			lst = append(lst, m)
		} else {
			lst = append(lst, t) // append origin txn if TxnUnmarshal fail
		}
	}
	m["transactions"] = lst

	return json.Marshal(m)
}
