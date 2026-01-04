package clique

import "github.com/ethereum/go-ethereum/common"

type RewardSnapshot struct {
	Signer    common.Address
	Receivers []common.Address
	Weights   []uint64
}
