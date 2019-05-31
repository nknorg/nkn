package websocket

import (
	"encoding/json"

	"github.com/nknorg/nkn/api/common"
	"github.com/nknorg/nkn/api/websocket/server"
	"github.com/nknorg/nkn/block"
	. "github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/event"
	"github.com/nknorg/nkn/node"
	"github.com/nknorg/nkn/util/config"
	"github.com/nknorg/nkn/vault"
)

var ws *server.WsServer

var (
	pushBlockFlag    bool = true
	pushRawBlockFlag bool = false
	pushBlockTxsFlag bool = false
)

func NewServer(localNode *node.LocalNode, w vault.Wallet) *server.WsServer {
	//	common.SetNode(n)
	event.Queue.Subscribe(event.NewBlockProduced, SendBlock2WSclient)
	ws = server.InitWsServer(localNode, w)
	return ws
}

func SendBlock2WSclient(v interface{}) {
	go PushSigChainBlockHash(v)
	if config.Parameters.HttpWsPort != 0 && pushBlockFlag {
		go PushBlock(v)
	}
	if config.Parameters.HttpWsPort != 0 && pushBlockTxsFlag {
		go PushBlockTransactions(v)
	}
}

func GetWsPushBlockFlag() bool {
	return pushBlockFlag
}

func SetWsPushBlockFlag(b bool) {
	pushBlockFlag = b
}

func GetPushRawBlockFlag() bool {
	return pushRawBlockFlag
}

func SetPushRawBlockFlag(b bool) {
	pushRawBlockFlag = b
}

func GetPushBlockTxsFlag() bool {
	return pushBlockTxsFlag
}

func SetPushBlockTxsFlag(b bool) {
	pushBlockTxsFlag = b
}

func SetTxHashMap(txhash string, sessionid string) {
	if ws == nil {
		return
	}
	ws.SetTxHashMap(txhash, sessionid)
}

func PushBlock(v interface{}) {
	if ws == nil {
		return
	}
	resp := common.ResponsePack(common.SUCCESS)
	if block, ok := v.(*block.Block); ok {
		if pushRawBlockFlag {
			dt, _ := block.Marshal()
			resp["Result"] = BytesToHexString(dt)
		} else {
			info, _ := block.GetInfo()
			var x interface{}
			json.Unmarshal(info, &x)
			resp["Result"] = x
		}
		resp["Action"] = "sendRawBlock"
		ws.PushResult(resp)
	}
}

func PushBlockTransactions(v interface{}) {
	if ws == nil {
		return
	}
	resp := common.ResponsePack(common.SUCCESS)
	if block, ok := v.(*block.Block); ok {
		if pushBlockTxsFlag {
			resp["Result"] = common.GetBlockTransactions(block)
		}
		resp["Action"] = "sendblocktransactions"
		ws.PushResult(resp)
	}
}

func PushSigChainBlockHash(v interface{}) {
	if ws == nil {
		return
	}
	resp := common.ResponsePack(common.SUCCESS)
	if block, ok := v.(*block.Block); ok {
		resp["Action"] = "updateSigChainBlockHash"
		resp["Result"] = BytesToHexString(block.Header.UnsignedHeader.PrevBlockHash)
		ws.PushResult(resp)
	}
}

func GetServer() *server.WsServer {
	return ws
}
