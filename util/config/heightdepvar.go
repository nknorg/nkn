package config

type HeightDependentInt struct {
	heights []uint32
	values  []int
}

func (hdi *HeightDependentInt) GetValueAtHeight(height uint32) int {
	for i, h := range hdi.heights {
		if height >= h {
			return hdi.values[i]
		}
	}
	return 0
}
