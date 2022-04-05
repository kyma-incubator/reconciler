package cluster

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/suite"
	"path/filepath"
	"sync"
	"testing"
)

type clusterTestSuite struct {
	suite.Suite
	containerSuite   *db.ContainerTestSuite
	testContext      context.Context
	serverStartMutex sync.Mutex
	debugLogs        bool
}

func TestIntegrationSuite(t *testing.T) {
	containerSettings := &db.PostgresContainerSettings{
		Name:              "default-db-shared",
		Image:             "postgres:11-alpine",
		Config:            db.MigrationConfig(filepath.Join("..", "..", "configs", "db", "postgres")),
		Host:              "127.0.0.1",
		Database:          "kyma",
		Port:              5432,
		User:              "kyma",
		Password:          "kyma",
		EncryptionKeyFile: filepath.Join("..", "..", "configs", "encryption", "unittest.key"),
	}
	cs := db.IsolatedContainerTestSuite(
		t,
		true,
		*containerSettings,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &clusterTestSuite{
		containerSuite: cs,
		testContext:    context.Background(),
		debugLogs:      true,
	})
	db.ReturnLeasedSharedContainerTestSuite(t, containerSettings)
}

func (s *clusterTestSuite) SetupSuite() {
	s.containerSuite.SetupSuite()
	s.serverStartMutex = sync.Mutex{}
}

func (s *clusterTestSuite) TearDownSuite() {
	s.containerSuite.TearDownSuite()
}

func (s *clusterTestSuite) TxConnection() *db.TxConnection {
	return s.containerSuite.TxConnection()
}

func (s *clusterTestSuite) NewConnection() (db.Connection, error) {
	return s.containerSuite.NewConnection()
}
