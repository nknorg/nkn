package config

import "github.com/nknorg/nkn/common"

type HeightDependentInt32 struct {
	heights []uint32
	values  []int32
}

func (hdi *HeightDependentInt32) GetValueAtHeight(height uint32) int32 {
	for i, h := range hdi.heights {
		if height >= h {
			return hdi.values[i]
		}
	}
	return 0
}

type HeightDependentUint256 struct {
	heights []uint32
	values  []common.Uint256
}

func (hdi *HeightDependentUint256) GetValueAtHeight(height uint32) common.Uint256 {
	for i, h := range hdi.heights {
		if height >= h {
			return hdi.values[i]
		}
	}
	return common.EmptyUint256
}
