package db

import (
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"strconv"
)

func BootstrapNewPostgresContainer(env postgresEnvironment, ctx context.Context) (ContainerBootstrap, error) {
	cont := NewPostgresContainer(env)
	if bootstrapError := cont.Bootstrap(ctx); bootstrapError != nil {
		return nil, bootstrapError
	}
	return &cont, nil
}

func NewPostgresContainer(env postgresEnvironment) PostgresContainer {
	return PostgresContainer{
		containerBaseName: "postgres",
		image:             "postgres:11-alpine",
		host:              env.host,
		port:              env.port,
		username:          env.user,
		password:          env.password,
		database:          env.database,
	}
}

//PostgresContainer is a testcontainer that is able to provision a postgres database with given credentials
type PostgresContainer struct {
	testcontainers.Container

	executionId  uuid.UUID
	bootstrapped bool

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

func (s *PostgresContainer) isBootstrapped() bool {
	return s.bootstrapped
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
			SkipReaper: true,
		},
		Started: true,
	})

	if requestError != nil {
		return requestError
	}

	s.Container = postgres
	s.bootstrapped = true
	return nil
}
