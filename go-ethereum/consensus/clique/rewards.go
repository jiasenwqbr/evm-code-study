package clique

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var rewardSetUpdatedSig = crypto.Keccak256Hash(
	[]byte("RewardSetUpdated(address,address[],uint256[])"),
)

func (c *Clique) ProcessRewardEvents(receipts types.Receipts) {
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			if len(log.Topics) == 0 {
				continue
			}
			if log.Topics[0] != rewardSetUpdatedSig {
				continue
			}

			signer := common.BytesToAddress(log.Topics[1].Bytes())

			var decoded struct {
				Receivers []common.Address
				Weights   []*big.Int
			}

			err := rewardABI.UnpackIntoInterface(
				&decoded,
				"RewardSetUpdated",
				log.Data,
			)
			if err != nil {
				continue
			}

			weights := make([]uint64, len(decoded.Weights))
			for i, w := range decoded.Weights {
				weights[i] = w.Uint64()
			}

			c.rewardSnapshots[signer] = &RewardSnapshot{
				Signer:    signer,
				Receivers: decoded.Receivers,
				Weights:   weights,
			}
		}
	}
}
