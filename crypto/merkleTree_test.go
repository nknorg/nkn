package crypto

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/nknorg/nkn/v2/common"
)

func TestHash(t *testing.T) {

	var data []common.Uint256
	a1 := common.Uint256(sha256.Sum256([]byte("a")))
	a2 := common.Uint256(sha256.Sum256([]byte("b")))
	a3 := common.Uint256(sha256.Sum256([]byte("c")))
	a4 := common.Uint256(sha256.Sum256([]byte("d")))
	a5 := common.Uint256(sha256.Sum256([]byte("e")))
	data = append(data, a1)
	data = append(data, a2)
	data = append(data, a3)
	data = append(data, a4)
	data = append(data, a5)
	x, _ := ComputeRoot(data)
	fmt.Printf("[Root Hash]:%x\n", x)

}
