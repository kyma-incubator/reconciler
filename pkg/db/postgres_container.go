package db

import (
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"strconv"
)

type ContainerBootstrap interface {
	testcontainers.Container
	Bootstrap(ctx context.Context) error
}

type PostgresContainer struct {
	testcontainers.Container

	executionId uuid.UUID
	Log         *zap.SugaredLogger

	containerBaseName string
	image             string
	host              string
	port              int
	username          string
	password          string
	database          string

	dataVolumeMapping string

	DebugLogs bool
}

func (s *PostgresContainer) Bootstrap(ctx context.Context) error {
	s.executionId = uuid.New()

	execContainer := s.containerBaseName + "-" + s.executionId.String()

	finalPortSpec := strconv.Itoa(s.port) + "/tcp"

	dbURL := func(port nat.Port) string {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			s.username, s.password, s.host, port.Port(), s.database,
		)
	}

	s.Log = log.NewLogger(s.DebugLogs).With("container", execContainer)
	postgres, requestError := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        s.image,
			ExposedPorts: []string{finalPortSpec},
			WaitingFor:   wait.ForSQL(nat.Port(strconv.Itoa(s.port)), "postgres", dbURL),
			Name:         execContainer,
			Labels:       map[string]string{"name": execContainer},
			Env: map[string]string{
				"POSTGRES_PASSWORD": s.password,
				"POSTGRES_USER":     s.username,
				"POSTGRES_DB":       s.database,
			},
			AutoRemove: true,
		},
		Started: true,
	})

	if requestError != nil {
		return requestError
	}

	s.Container = postgres

	return nil
}
