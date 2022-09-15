package btc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

func getBlock(ctx context.Context, client *resty.Client, param string) (*Block, error) {
	resp, err := client.R().SetContext(ctx).Get(fmt.Sprintf("https://chain.api.btc.com/v3/block/%v", param))
	if err != nil {
		return nil, errors.Wrapf(err, "problem fetching block: %v", param)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %v", resp.StatusCode())
	}
	var blockResp *BlockResp
	body := resp.Body()
	if err = json.Unmarshal(body, &blockResp); err != nil {
		return nil, errors.Wrapf(err, "problem parsing block response for block: %v", param)
	}
	if !blockResp.IsValid() {
		return nil, fmt.Errorf("invalid block response")
	}
	return blockResp.Data, nil
}

func GetBlock(ctx context.Context, client *resty.Client, height int64) (*Block, error) {
	return getBlock(ctx, client, strconv.FormatInt(height, 10))
}

func GetLatestBlock(ctx context.Context, client *resty.Client) (*Block, error) {
	return getBlock(ctx, client, "latest")
}
