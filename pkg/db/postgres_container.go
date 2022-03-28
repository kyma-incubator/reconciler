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

func BootstrapNewPostgresContainer(ctx context.Context, settings PostgresContainerSettings) (ContainerBootstrap, error) {
	cont := NewPostgresContainer(settings)
	if bootstrapError := cont.Bootstrap(ctx); bootstrapError != nil {
		return nil, bootstrapError
	}
	return &cont, nil
}

func NewPostgresContainer(settings PostgresContainerSettings) PostgresContainer {
	return PostgresContainer{
		containerBaseName: settings.name,
		image:             settings.image,
		host:              settings.host,
		port:              settings.port,
		username:          settings.user,
		password:          settings.password,
		database:          settings.database,
	}
}

//PostgresContainer is a testcontainer that is able to provision a postgres database with given credentials
type PostgresContainer struct {
	testcontainers.Container

	executionID  uuid.UUID
	bootstrapped bool

	containerBaseName string
	image             string
	host              string
	port              int
	username          string
	password          string
	database          string

	DebugLogs bool
}

func (s *PostgresContainer) isBootstrapped() bool {
	return s.bootstrapped
}

func (s *PostgresContainer) ExecutionID() string {
	return s.executionID.String()
}

func (s *PostgresContainer) Bootstrap(ctx context.Context) error {
	s.executionID = uuid.New()

	execContainer := s.containerBaseName + "-" + s.executionID.String()

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
