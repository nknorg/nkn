package chain

import (
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/util/config"
)

var (
	MiningRewards = make(map[uint32]common.Fixed64)
)

func init() {
	for i := uint32(0); i < config.TotalRewardDuration; i++ {
		MiningRewards[i] = common.Fixed64(config.InitialReward - config.ReductionAmount*common.Fixed64(i))
	}
}

func GetRewardByHeight(height uint32) common.Fixed64 {
	if height == 0 {
		panic("invalid height")
	}
	duration := (height - 1) / uint32(config.RewardAdjustInterval)
	if duration >= config.TotalRewardDuration {
		return 0
	}

	reward := MiningRewards[duration]
	rewardPerBlock := int64(reward) / int64(config.RewardAdjustInterval)
	return common.Fixed64(rewardPerBlock)
}
