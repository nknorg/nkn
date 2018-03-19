package ledger

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	. "nkn-core/common"
	"nkn-core/common/serialization"
	"nkn-core/core/contract/program"
	sig "nkn-core/core/signature"
	. "nkn-core/errors"
)

type BlockHeader struct {
	Version          uint32
	PrevBlockHash    Uint256
	TransactionsRoot Uint256
	Timestamp        uint32
	Bits             uint32
	Height           uint32
	Nonce            uint32
	Program          *program.Program

	hash Uint256
}

//Serialize the blockheader
func (bd *BlockHeader) Serialize(w io.Writer) {
	bd.SerializeUnsigned(w)
	w.Write([]byte{byte(1)})
	if bd.Program != nil {
		bd.Program.Serialize(w)
	}
}

func (bd *BlockHeader) SerializeUnsigned(w io.Writer) error {
	serialization.WriteUint32(w, bd.Version)
	bd.PrevBlockHash.Serialize(w)
	bd.TransactionsRoot.Serialize(w)
	serialization.WriteUint32(w, bd.Timestamp)
	serialization.WriteUint32(w, bd.Height)
	serialization.WriteUint32(w, bd.Nonce)
	return nil
}

func (bd *BlockHeader) Deserialize(r io.Reader) error {
	bd.DeserializeUnsigned(r)
	p := make([]byte, 1)
	n, err := r.Read(p)
	if n > 0 {
		x := []byte(p[:])

		if x[0] != byte(1) {
			return NewDetailErr(errors.New("BlockHeader Deserialize get format error."), ErrNoCode, "")
		}
	} else {
		return NewDetailErr(errors.New("BlockHeader Deserialize get format error."), ErrNoCode, "")
	}

	pg := new(program.Program)
	err = pg.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "BlockHeader item Program Deserialize failed.")
	}
	bd.Program = pg
	return nil
}

func (bd *BlockHeader) DeserializeUnsigned(r io.Reader) error {
	//Version
	temp, err := serialization.ReadUint32(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "BlockHeader item Version Deserialize failed.")
	}
	bd.Version = temp

	//PrevBlockHash
	preBlock := new(Uint256)
	err = preBlock.Deserialize(r)
	if err != nil {
		return NewDetailErr(err, ErrNoCode, "BlockHeader item preBlock Deserialize failed.")
	}
	bd.PrevBlockHash = *preBlock

	//TransactionsRoot
	txRoot := new(Uint256)
	err = txRoot.Deserialize(r)
	if err != nil {
		return err
	}
	bd.TransactionsRoot = *txRoot

	//Timestamp
	temp, _ = serialization.ReadUint32(r)
	bd.Timestamp = uint32(temp)

	//Height
	temp, _ = serialization.ReadUint32(r)
	bd.Height = uint32(temp)

	//Nonce
	bd.Nonce, _ = serialization.ReadUint32(r)

	return nil
}

func (bd *BlockHeader) GetProgramHashes() ([]Uint160, error) {
	return nil, nil
}

func (bd *BlockHeader) SetPrograms(programs []*program.Program) {
}

func (bd *BlockHeader) GetPrograms() []*program.Program {
	return nil
}

func (bd *BlockHeader) Hash() Uint256 {
	d := sig.GetHashData(bd)
	temp := sha256.Sum256([]byte(d))
	f := sha256.Sum256(temp[:])
	hash := Uint256(f)
	return hash
}

func (bd *BlockHeader) GetMessage() []byte {
	return sig.GetHashData(bd)
}

func (bd *BlockHeader) ToArray() []byte {
	b := new(bytes.Buffer)
	bd.Serialize(b)
	return b.Bytes()
}
