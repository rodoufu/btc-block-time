package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

func GetBlocksForDay(ctx context.Context, client *resty.Client, day time.Time) ([]*Block, error) {
	resp, err := client.R().SetContext(ctx).
		Get(fmt.Sprintf("https://blockchain.info/blocks/%v?format=json", day.UnixMilli()))
	if err != nil {
		return nil, errors.Wrapf(err, "problem fetching blocks for day %v", day)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %v", resp.StatusCode())
	}
	var blocks []*Block
	body := resp.Body()
	if err = json.Unmarshal(body, &blocks); err != nil {
		return nil, errors.Wrapf(err, "problem parsing blocks response for day %v", day)
	}
	return blocks, nil
}
