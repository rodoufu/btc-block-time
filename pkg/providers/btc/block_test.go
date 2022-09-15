package btc

import (
	"reflect"
	"testing"
	"time"

	"github.com/rodoufu/btc-block-time/pkg/entity"
)

func TestBlockResp_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		blockResp *BlockResp
		want      bool
	}{
		{
			name: "empty",
			want: false,
		},
		{
			name:      "no blocks",
			blockResp: &BlockResp{},
			want:      false,
		},
		{
			name: "one blocks",
			blockResp: &BlockResp{
				Data: &Block{},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.blockResp.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlock_ToBlock(t *testing.T) {
	t.Parallel()
	now := time.UnixMilli(time.Now().UnixMilli() / 1000 * 1000)
	tests := []struct {
		name  string
		block *Block
		want  *entity.Block
	}{
		{
			name: "empty",
		},
		{
			name: "simple",
			block: &Block{
				Height:    10,
				Hash:      "abc",
				Timestamp: now.UnixMilli() / 1000,
			},
			want: &entity.Block{
				Height:    10,
				Hash:      "abc",
				Timestamp: now,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.block.ToBlock(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
