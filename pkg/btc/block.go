package btc

import "time"

type Block struct {
	Height    int64  `json:"height"`
	Hash      string `json:"hash"`
	Timestamp int64  `json:"timestamp"`
}

func (b *Block) GetTime() time.Time {
	return time.UnixMilli(b.Timestamp * 1000)
}

type BlockResp struct {
	Data    *Block `json:"data"`
	ErrCode int    `json:"err_code"`
	ErrNo   int    `json:"err_no"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func (r *BlockResp) IsValid() bool {
	return r.ErrCode == 0 && r.Data != nil
}
