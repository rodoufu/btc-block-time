package btc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
)

func GetBlock(ctx context.Context, client *resty.Client, height int64) (*Block, error) {
	resp, err := client.R().SetContext(ctx).Get(fmt.Sprintf("https://chain.api.btc.com/v3/block/%v", height))
	if err != nil {
		return nil, errors.Wrapf(err, "problem fetching height: %v", height)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %v", resp.StatusCode())
	}
	var blockResp *BlockResp
	if err = json.Unmarshal(resp.Body(), &blockResp); err != nil {
		return nil, errors.Wrapf(err, "problem parsing block response for height: %v", height)
	}
	if blockResp == nil || !blockResp.IsValid() {
		return nil, fmt.Errorf("invalid block response")
	}
	return blockResp.Data, nil
}
