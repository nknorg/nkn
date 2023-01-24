package config

import "github.com/nknorg/nkn/v2/common"

type HeightDependentInt32 struct {
	Height []uint32
	Values []int32
}

func (hdi *HeightDependentInt32) GetValueAtHeight(height uint32) int32 {
	for i, h := range hdi.Height {
		if height >= h {
			return hdi.Values[i]
		}
	}
	return 0
}

type HeightDependentInt64 struct {
	Height []uint32
	Values []int64
}

func (hdi *HeightDependentInt64) GetValueAtHeight(height uint32) int64 {
	for i, h := range hdi.Height {
		if height >= h {
			return hdi.Values[i]
		}
	}
	return 0
}

type HeightDependentUint256 struct {
	Height []uint32
	Values []common.Uint256
}

func (hdi *HeightDependentUint256) GetValueAtHeight(height uint32) common.Uint256 {
	for i, h := range hdi.Height {
		if height >= h {
			return hdi.Values[i]
		}
	}
	return common.EmptyUint256
}

type HeightDependentBool struct {
	Heights []uint32
	Values  []bool
}

func (hdi *HeightDependentBool) GetValueAtHeight(height uint32) bool {
	for i, h := range hdi.Heights {
		if height >= h {
			return hdi.Values[i]
		}
	}
	return false
}

type HeightDependentString struct {
	Height []uint32
	Values []string
}

func (hdi *HeightDependentString) GetValueAtHeight(height uint32) string {
	for i, h := range hdi.Height {
		if height >= h {
			return hdi.Values[i]
		}
	}
	return ""
}
