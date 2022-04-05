package reconciliation

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"path/filepath"
	"sync"
	"testing"
)

type reconciliationTestSuite struct {
	suite.Suite
	containerSuite   *db.ContainerTestSuite
	testContext      context.Context
	serverStartMutex sync.Mutex
	debugLogs        bool
}

type testEntities struct {
	persistenceRepo          Repository
	inMemoryRepo             Repository
	persistenceSchedulingIDs []string
	inMemorySchedulingIDs    []string
	runtimeIDs               []string
	teardownFn               func()
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

func (s *reconciliationTestSuite) prepareTest(t *testing.T, count int) *testEntities {
	//create mock database connection
	dbConn = s.dbTestConnection()

	persistenceSchedulingIDs := make([]string, 0, count)
	inMemorySchedulingIDs := make([]string, 0, count)

	persistenceRepo, err := NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)
	inMemoryRepo := NewInMemoryReconciliationRepository()

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	var runtimeIDs []string
	for i := 0; i < count; i++ {
		//add cluster(s) to inventory
		clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "1", 1, false, test.OneComponentDummy))
		require.NoError(t, err)

		//collect runtimeIDs for cleanup
		runtimeIDs = append(runtimeIDs, clusterState.Cluster.RuntimeID)

		//trigger reconciliation for cluster
		persistenceReconEntity, err := persistenceRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		require.NoError(t, err)
		inMemoryReconEntity, err := inMemoryRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		require.NoError(t, err)

		// collect schedulingIDs for deletion
		persistenceSchedulingIDs = append(persistenceSchedulingIDs, persistenceReconEntity.SchedulingID)
		inMemorySchedulingIDs = append(inMemorySchedulingIDs, inMemoryReconEntity.SchedulingID)
	}

	// clean-up created cluster
	teardownFn := func() {
		for _, runtimeID := range runtimeIDs {
			require.NoError(t, persistenceRepo.RemoveReconciliationByRuntimeID(runtimeID))
			require.NoError(t, inventory.Delete(runtimeID))
		}
	}

	return &testEntities{persistenceRepo, inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs, runtimeIDs, teardownFn}
}
