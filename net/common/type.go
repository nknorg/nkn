package common

type NodeInfo struct {
	State     uint32 `json:"State"`     // node status
	SyncState string `json:"SyncState"` // node block sync status
	Port      uint16 `json:"Port"`      // The nodes's port
	NodePort  uint16 `json:"NodePort"`  // The nodes's port
	JsonPort  uint16 `json:"JsonPort"`  // The node's RPC httpjson port
	WsPort    uint16 `json:"WsPort"`    // The node's RPC Ws port
	Addr      string `json:"Addr"`      // The node's IP address
	ID        uint64 `json:"ID"`        // The nodes's id
	Time      int64  `json:"Time"`
	Version   uint32 `json:"Version"`  // The network protocol the node used
	Relay     bool   `json:"Relay"`    // The relay capability of the node (merge into capbility flag)
	Height    uint32 `json:"Height"`   // The node latest block height
	PubKey    string `json:"PubKey"`   // The node Public Key
	TxnCnt    uint64 `json:"TxnCnt"`   // The transactions be transmit by this node
	RxTxnCnt  uint64 `json:"RxTxnCnt"` // The transaction received by this node
	ChordID   string `json:"ChordID"`  // Chord ID
}
