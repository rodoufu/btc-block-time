package blockchain

import (
	"time"

	"github.com/rodoufu/btc-block-time/pkg/entity"
)

type Block struct {
	Height    int64  `json:"height"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"time"`
}

func (b *Block) ToBlock() *entity.Block {
	if b == nil {
		return nil
	}
	return &entity.Block{
		Height:    b.Height,
		Hash:      b.Hash,
		Timestamp: time.UnixMilli(b.Timestamp * 1000),
	}
}

type BlockResp struct {
	Blocks []*Block `json:"blocks"`
}

func (r *BlockResp) IsValid() bool {
	return r != nil && len(r.Blocks) > 0
}
