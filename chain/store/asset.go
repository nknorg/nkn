package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
)

type Asset struct {
	name        string
	symbol      string
	totalSupply common.Fixed64
	precision   uint32
	owner       common.Uint160
}

func (ia *Asset) Empty() bool {
	return len(ia.name) == 0 && len(ia.symbol) == 0
}

func (ia *Asset) Serialize(w io.Writer) error {
	if err := serialization.WriteVarString(w, ia.name); err != nil {
		return err
	}

	if err := serialization.WriteVarString(w, ia.symbol); err != nil {
		return err
	}

	if err := ia.totalSupply.Serialize(w); err != nil {
		return err
	}

	if err := serialization.WriteUint32(w, ia.precision); err != nil {
		return err
	}

	if _, err := ia.owner.Serialize(w); err != nil {
		return err
	}

	return nil
}

func (ia *Asset) Deserialize(r io.Reader) error {
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

	ia.precision, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	err = ia.owner.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}

func (sdb *StateDB) SetAsset(assetID common.Uint256, name string, symbol string, totalSupply common.Fixed64, precision uint32, owner common.Uint160) error {
	asset := &Asset{
		name:        name,
		symbol:      symbol,
		totalSupply: totalSupply,
		precision:   precision,
		owner:       owner,
	}

	return sdb.setAsset(assetID, asset)
}

func (sdb *StateDB) setAsset(assetID common.Uint256, asset *Asset) error {
	if asset, err := sdb.getAsset(assetID); err == nil && asset != nil {
		return fmt.Errorf("asset %v has already exist.", assetID.ToHexString())
	}

	sdb.assets.Store(assetID, asset)

	return nil
}

func (sdb *StateDB) increaseTotalSupply(assetID common.Uint256, amount common.Fixed64) error {
	asset, err := sdb.GetAsset(assetID)
	if err != nil {
		return err
	}

	if asset.totalSupply+amount <= 0 {
		return errors.New("totalSupply overflow")
	}

	asset.totalSupply += amount

	return nil
}

func (sdb *StateDB) updateAsset(assetID common.Uint256, asset *Asset) error {
	buff := bytes.NewBuffer(nil)
	err := asset.Serialize(buff)
	if err != nil {
		return err
	}

	return sdb.trie.TryUpdate(append(IssueAssetPrefix, assetID[:]...), buff.Bytes())
}

func (sdb *StateDB) GetAsset(assetID common.Uint256) (*Asset, error) {
	if v, ok := sdb.assets.Load(assetID); ok {
		if asset, ok := v.(*Asset); ok {
			return asset, nil
		}
	}

	return sdb.getAsset(assetID)
}

func (sdb *StateDB) getAsset(assetID common.Uint256) (*Asset, error) {
	enc, err := sdb.trie.TryGet(append(IssueAssetPrefix, assetID[:]...))
	if err != nil || len(enc) == 0 {
		return nil, err
	}

	asset := &Asset{}
	buff := bytes.NewBuffer(enc)
	if err := asset.Deserialize(buff); err != nil {
		return nil, err
	}

	sdb.assets.Store(assetID, asset)

	return asset, nil
}

func (sdb *StateDB) deleteAsset(assetID common.Uint256) error {
	sdb.assets.Delete(assetID)

	return nil
}

func (sdb *StateDB) FinalizeIssueAsset(commit bool) {
	sdb.assets.Range(func(key, value interface{}) bool {
		if assetID, ok := key.(common.Uint256); ok {
			if asset, ok := value.(*Asset); ok && !asset.Empty() {
				sdb.updateAsset(assetID, asset)
			} else {
				sdb.deleteAsset(assetID)
			}
			if commit {
				sdb.assets.Delete(assetID)
			}
		}
		return true
	})
}

func (cs *ChainStore) GetAsset(assetID common.Uint256) (name, symbol string, totalSupply common.Fixed64, precision uint32, err error) {
	asset, err := cs.States.getAsset(assetID)
	if err != nil {
		return "", "", 0, 0, err
	}
	if asset == nil {
		return "", "", 0, 0, errors.New("asset in GetAsset is nil")
	}

	return asset.name, asset.symbol, asset.totalSupply, asset.precision, nil
}
