package reconciliation

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
)

type reconciliationTestSuite struct {
	suite.Suite
	containerSuite   *db.ContainerTestSuite
	testContext      context.Context
	serverStartMutex sync.Mutex
	debugLogs        bool
	runtimeIDs       []string
	inventory        cluster.Inventory
	persistenceRepo  Repository
}

type testEntities struct {
	inMemoryRepo             Repository
	persistenceSchedulingIDs []string
	inMemorySchedulingIDs    []string
}

func TestIntegrationSuite(t *testing.T) {
	cs := db.IsolatedContainerTestSuite(
		t,
		true,
		*db.DefaultSharedContainerSettings,
		false,
	)
	cs.SetT(t)
	suite.Run(t, &reconciliationTestSuite{
		containerSuite: cs,
		testContext:    context.Background(),
		debugLogs:      true,
	})
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

func (s *reconciliationTestSuite) prepareTest(t *testing.T, count int) *testEntities {
	var err error
	//create mock database connection
	dbConn = s.TxConnection()

	persistenceSchedulingIDs := make([]string, 0, count)
	inMemorySchedulingIDs := make([]string, 0, count)

	s.persistenceRepo, err = NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)
	inMemoryRepo := NewInMemoryReconciliationRepository()

	//prepare inventory
	s.inventory, err = cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	for i := 0; i < count; i++ {
		//add cluster(s) to inventory
		clusterState, err := s.inventory.CreateOrUpdate(1, test.NewCluster(t, "1", 1, false, test.OneComponentDummy))
		require.NoError(t, err)

		//collect runtimeIDs for cleanup
		s.runtimeIDs = append(s.runtimeIDs, clusterState.Cluster.RuntimeID)

		//trigger reconciliation for cluster
		persistenceReconEntity, err := s.persistenceRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		require.NoError(t, err)
		inMemoryReconEntity, err := inMemoryRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		require.NoError(t, err)

		// collect schedulingIDs for deletion
		persistenceSchedulingIDs = append(persistenceSchedulingIDs, persistenceReconEntity.SchedulingID)
		inMemorySchedulingIDs = append(inMemorySchedulingIDs, inMemoryReconEntity.SchedulingID)
	}

	return &testEntities{inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs}
}

func (s *reconciliationTestSuite) BeforeTest(suiteName, testName string) {
	s.runtimeIDs = nil
	s.inventory = nil
	s.persistenceRepo = nil
}

func (s *reconciliationTestSuite) AfterTest(suiteName, testName string) {
	t := s.T()
	for _, runtimeID := range s.runtimeIDs {
		require.NoError(t, s.persistenceRepo.RemoveReconciliationByRuntimeID(runtimeID))
		require.NoError(t, s.inventory.Delete(runtimeID))
	}
}
