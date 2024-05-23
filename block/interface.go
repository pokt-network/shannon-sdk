package block

import (
	"context"
)

type BlockClient interface {
	GetLatestBlockHeight(ctx context.Context) (height int64, err error)
}
