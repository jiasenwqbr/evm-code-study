package clique

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

var rewardABI abi.ABI

func init() {
	const rewardABIJSON = `[
        {
            "anonymous": false,
            "inputs": [
                {
                    "indexed": true,
                    "internalType": "address",
                    "name": "signer",
                    "type": "address"
                },
                {
                    "indexed": false,
                    "internalType": "address[]",
                    "name": "receivers",
                    "type": "address[]"
                },
                {
                    "indexed": false,
                    "internalType": "uint256[]",
                    "name": "weights",
                    "type": "uint256[]"
                }
            ],
            "name": "RewardSetUpdated",
            "type": "event"
        }
    ]`

	parsed, err := abi.JSON(strings.NewReader(rewardABIJSON))
	if err != nil {
		panic(err)
	}
	rewardABI = parsed
}
