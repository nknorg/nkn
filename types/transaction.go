package types

import "crypto/sha256"

func (tx *MetaTx) Hash() Uint256 {
	marshaledTx, _ := tx.Marshal()

	midHash := sha256.Sum256(marshaledTx)
	hash := Uint256(sha256.Sum256(midHash[:]))

	return hash
}

func (tx *Transaction) Hash() Uint256 {
	return tx.MetaTx.Hash()
}

func (tx *Transaction) Sign() {
}
