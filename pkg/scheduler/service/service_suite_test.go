package service

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"sync"
	"testing"
)

type serviceTestSuite struct {
	suite.Suite
	containerSuite    *db.ContainerTestSuite
	testContext       context.Context
	testLogger        *zap.SugaredLogger
	serverStartMutex  sync.Mutex
	debugLogs         bool
	inventory         cluster.Inventory
	runtimeIDsToClear []string
	reconRepo         reconciliation.Repository
	dbConn            db.Connection
}

func TestIntegrationSuite(t *testing.T) {
	cs := db.IsolatedContainerTestSuite(
		t,
		true,
		*db.DefaultSharedContainerSettings,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &serviceTestSuite{
		containerSuite: cs,
		testContext:    context.Background(),
		testLogger:     logger.NewLogger(true),
		debugLogs:      true,
	})
}

func (s *serviceTestSuite) SetupSuite() {
	s.containerSuite.SetupSuite()
	s.serverStartMutex = sync.Mutex{}
}

func (s *serviceTestSuite) TearDownSuite() {
	s.containerSuite.TearDownSuite()
}

func (s *serviceTestSuite) TxConnection() *db.TxConnection {
	return s.containerSuite.TxConnection()
}

func (s *serviceTestSuite) NewConnection() (db.Connection, error) {
	return s.containerSuite.NewConnection()
}

func (s *serviceTestSuite) BeforeTest(suiteName, testName string) {
	s.inventory = nil
	s.dbConn = nil
	s.runtimeIDsToClear = []string{}
	s.reconRepo = nil
}

func (s *serviceTestSuite) AfterTest(suiteName, testName string) {
	t := s.T()
	for _, runtimeID := range s.runtimeIDsToClear {
		require.NoError(t, s.inventory.Delete(runtimeID))
		recons, err := s.reconRepo.GetReconciliations(&reconciliation.WithRuntimeID{RuntimeID: runtimeID})
		require.NoError(t, err)
		for _, recon := range recons {
			require.NoError(t, s.reconRepo.RemoveReconciliationBySchedulingID(recon.SchedulingID))
		}
		require.NoError(t, s.dbConn.Close())
	}
}
