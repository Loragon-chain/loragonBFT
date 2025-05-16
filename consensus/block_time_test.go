package consensus

import (
	"fmt"
	"time"

	"github.com/Loragon-chain/loragonBFT/block"
	"github.com/Loragon-chain/loragonBFT/genesis"
	"github.com/Loragon-chain/loragonBFT/libs/kv"
	"github.com/ethereum/go-ethereum/common"
)

var (
	TestAddr = common.HexToAddress("0x7567d83b7b8d80addcb281a71d54fc7b3364ffed")
)

func buildGenesis(kv kv.GetPutter, proc func() error) *block.Block {
	blk, err := new(genesis.Builder).
		Timestamp(uint64(time.Now().Unix())).
		Build()
	if err != nil {
		fmt.Println("ERROR: ", err)
	}
	return blk
}
