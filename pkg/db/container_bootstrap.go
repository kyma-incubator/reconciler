package db

import (
	"context"
	"github.com/testcontainers/testcontainers-go"
)

//ContainerBootstrap is a testcontainer that can be bootstrapped and started from a given context
type ContainerBootstrap interface {
	testcontainers.Container
	Bootstrap(ctx context.Context) error
	isBootstrapped() bool
}
