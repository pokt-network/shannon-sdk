package session

import (
	"context"

	"github.com/pokt-network/poktroll/x/session/types"
)

type SessionClient interface {
	GetSession(
		ctx context.Context,
		appAddress string,
		serviceId string,
		height int64,
	) (session *types.Session, err error)
}
