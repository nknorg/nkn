package por

import (
	"io"

	"github.com/nknorg/nkn/common"
)

type porer interface {
	Hash() common.Uint256
	GetHeight() uint32
	GetTxHash() *common.Uint256
	GetSigChain() *SigChain
	CompareTo(p *porer) int
	Serialize(w io.Writer) error
	Deserialize(r io.Reader) error
}
