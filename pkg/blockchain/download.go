package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

func GetBlock(ctx context.Context, client *resty.Client, height int64) (*Block, error) {
	resp, err := client.R().SetContext(ctx).
		Get(fmt.Sprintf("https://blockchain.info/block-height/%v?format=json", height))
	if err != nil {
		return nil, errors.Wrapf(err, "problem fetching block: %v", height)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %v", resp.StatusCode())
	}
	var blockResp *BlockResp
	body := resp.Body()
	if err = json.Unmarshal(body, &blockResp); err != nil {
		return nil, errors.Wrapf(err, "problem parsing block response for block: %v", height)
	}
	if blockResp == nil || !blockResp.IsValid() {
		return nil, fmt.Errorf("invalid block response")
	}
	return blockResp.Blocks[0], nil
}
