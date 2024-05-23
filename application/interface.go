package application

import (
	"context"

	"github.com/pokt-network/poktroll/x/application/types"
)

type ApplicationClient interface {
	GetAllApplications(
		ctx context.Context,
	) ([]types.Application, error)

	GetApplication(
		ctx context.Context,
		appAddress string,
	) (types.Application, error)
}
