package db

import (
	"bytes"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type issueAsset struct {
	name        string
	symbol      string
	totalSupply common.Fixed64
	precision   byte
	owner       common.Uint160
}

func (ia *issueAsset) Empty() bool {
	return len(ia.name) == 0 && len(ia.symbol) == 0
}

func (ia *issueAsset) Serialize(w io.Writer) error {
	if err := serialization.WriteVarString(w, ia.name); err != nil {
		return err
	}

	if err := serialization.WriteVarString(w, ia.symbol); err != nil {
		return err
	}

	if err := ia.totalSupply.Serialize(w); err != nil {
		return err
	}

	if err := serialization.WriteByte(w, ia.precision); err != nil {
		return err
	}

	if _, err := ia.owner.Serialize(w); err != nil {
		return err
	}

	return nil
}

func (ia *issueAsset) Deserialize(r io.Reader) error {
	var err error

	ia.name, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	ia.symbol, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	err = ia.totalSupply.Deserialize(r)
	if err != nil {
		return err
	}

	ia.precision, err = serialization.ReadByte(r)
	if err != nil {
		return err
	}

	err = ia.owner.Deserialize(r)
	if err != nil {
		return err
	}
	return nil
}

func (sdb *StateDB) SetAsset(assetID common.Uint256, name string, symbol string, totalSupply common.Fixed64, precision byte, owner common.Uint160) error {
	asset := &issueAsset{
		name:        name,
		symbol:      symbol,
		totalSupply: totalSupply,
		precision:   precision,
		owner:       owner,
	}

	return sdb.setAsset(assetID, asset)
}

func (sdb *StateDB) setAsset(assetID common.Uint256, asset *issueAsset) error {
	if _, err := sdb.getAsset(assetID); err == nil {
		return fmt.Errorf("asset %v has already exist.", assetID.ToHexString())
	}

	sdb.assets[assetID] = asset

	return nil
}

func (sdb *StateDB) updateAsset(assetID common.Uint256, asset *issueAsset) error {
	buff := bytes.NewBuffer(nil)
	err := asset.Serialize(buff)
	if err != nil {
		return err
	}

	return sdb.trie.TryUpdate(append(IssueAssetPrefix, assetID[:]...), buff.Bytes())
}

func (sdb *StateDB) getAsset(assetID common.Uint256) (*issueAsset, error) {
	if asset, ok := sdb.assets[assetID]; ok {
		return asset, nil
	}

	enc, err := sdb.trie.TryGet(append(IssueAssetPrefix, assetID[:]...))
	if err != nil || len(enc) == 0 {
		return nil, err
	}

	asset := &issueAsset{}
	buff := bytes.NewBuffer(enc)
	if err := asset.Deserialize(buff); err != nil {
		return nil, err
	}

	sdb.assets[assetID] = asset

	return asset, nil
}

func (sdb *StateDB) deleteAsset(assetID common.Uint256) error {
	delete(sdb.assets, assetID)

	return nil
}

func (sdb *StateDB) FinalizeIssueAsset(commit bool) {
	for assetID, asset := range sdb.assets {
		if asset == nil || asset.Empty() {
			sdb.deleteAsset(assetID)
		} else {
			sdb.updateAsset(assetID, asset)
		}

		if commit {
			delete(sdb.assets, assetID)
		}
	}
}
