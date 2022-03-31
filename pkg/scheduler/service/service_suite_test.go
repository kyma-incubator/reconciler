package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"path/filepath"
	"sync"
	"testing"
)

type reconciliationTestSuite struct {
	suite.Suite
	containerSuite   *db.ContainerTestSuite
	testContext      context.Context
	testLogger       *zap.SugaredLogger
	serverStartMutex sync.Mutex
	debugLogs        bool
}

func TestIntegrationSuite(t *testing.T) {
	containerSettings := &db.PostgresContainerSettings{
		Name:              "default-db-shared",
		Image:             "postgres:11-alpine",
		Config:            db.MigrationConfig(filepath.Join("..", "..", "..", "configs", "db", "postgres")),
		Host:              "127.0.0.1",
		Database:          "kyma",
		Port:              5432,
		User:              "kyma",
		Password:          "kyma",
		EncryptionKeyFile: filepath.Join("..", "..", "..", "configs", "encryption", "unittest.key"),
	}
	cs := db.IsolatedContainerTestSuite(
		t,
		true,
		*containerSettings,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &reconciliationTestSuite{
		containerSuite: cs,
		testContext:    context.Background(),
		testLogger:     logger.NewLogger(true),
		debugLogs:      true,
	})
	db.ReturnLeasedSharedContainerTestSuite(t, containerSettings)
}

func (s *reconciliationTestSuite) SetupSuite() {
	s.containerSuite.SetupSuite()
	s.serverStartMutex = sync.Mutex{}
}

func (s *reconciliationTestSuite) TearDownSuite() {
	s.containerSuite.TearDownSuite()
}

func (s *reconciliationTestSuite) TxConnection() *db.TxConnection {
	return s.containerSuite.TxConnection()
}

func (s *reconciliationTestSuite) NewConnection() (db.Connection, error) {
	return s.containerSuite.NewConnection()
}
