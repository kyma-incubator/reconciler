package cluster

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/suite"
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
	cs := db.IsolatedContainerTestSuite(
		t,
		true,
		*db.DefaultSharedContainerSettings,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &clusterTestSuite{
		containerSuite: cs,
		testContext:    context.Background(),
		debugLogs:      true,
	})
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
