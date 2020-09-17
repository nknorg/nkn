package websocket

import (
	"encoding/hex"
	"encoding/json"

	"github.com/nknorg/nkn/v2/api/common"
	"github.com/nknorg/nkn/v2/api/common/errcode"
	"github.com/nknorg/nkn/v2/api/websocket/server"
	"github.com/nknorg/nkn/v2/block"
	"github.com/nknorg/nkn/v2/chain"
	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/event"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/util/log"
	"github.com/nknorg/nkn/v2/vault"
)

var ws *server.WsServer

var (
	pushBlockFlag    bool = false
	pushRawBlockFlag bool = false
	pushBlockTxsFlag bool = false
)

func NewServer(localNode *node.LocalNode, w *vault.Wallet) *server.WsServer {
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
	resp := common.ResponsePack(errcode.SUCCESS)
	if block, ok := v.(*block.Block); ok {
		if pushRawBlockFlag {
			dt, _ := block.Marshal()
			resp["Result"] = hex.EncodeToString(dt)
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
	resp := common.ResponsePack(errcode.SUCCESS)
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
	resp := common.ResponsePack(errcode.SUCCESS)
	if block, ok := v.(*block.Block); ok {
		sigChainBlockHeight := block.Header.UnsignedHeader.Height - config.SigChainBlockDelay
		sigChainBlockHash, err := chain.DefaultLedger.Store.GetBlockHash(sigChainBlockHeight)
		if err != nil {
			log.Warningf("get sigchain block hash at height %d error: %v", sigChainBlockHeight, err)
			return
		}
		resp["Action"] = "updateSigChainBlockHash"
		resp["Result"] = hex.EncodeToString(sigChainBlockHash.ToArray())
		ws.PushResult(resp)
	}
}

func GetServer() *server.WsServer {
	return ws
}
