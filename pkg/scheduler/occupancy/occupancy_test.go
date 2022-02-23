package occupancy

import (
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

var (
	dbConn db.Connection
	mu     sync.Mutex
)

type Occupancy struct {
	PoolID    string
	Component string
	Capacity  int
}

type testCase struct {
	name    string
	testFct func(t *testing.T, occupRepo Repository)
}

func TestOccupancyRepository(t *testing.T) {
	test.IntegrationTest(t)
	occupancies := []*Occupancy{
		{
			PoolID:    "1",
			Component: "cp1",
			Capacity:  50,
		},
		{
			PoolID:    "2",
			Component: "cp2",
			Capacity:  100,
		},
		{
			PoolID:    "3",
			Component: "cp3",
			Capacity:  150,
		},
	}

	testCases := []testCase{
		{
			"create occupancy with 0 running workers",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := uuid.NewString()
				occupEntity, err := occupancyRepo.CreateWorkerPoolOccupancy(poolID, "component1", 0, 50)
				require.NoError(t, err)
				require.Equal(t, poolID, occupEntity.WorkerPoolID)
				require.Equal(t, 50, int(occupEntity.WorkerPoolCapacity))
				require.Equal(t, 0, int(occupEntity.RunningWorkers))
			},
		},
		{
			"create occupancy with 10 running workers",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := uuid.NewString()
				occupEntity, err := occupancyRepo.CreateWorkerPoolOccupancy(poolID, "component1", 10, 50)
				require.NoError(t, err)
				require.Equal(t, poolID, occupEntity.WorkerPoolID)
				require.Equal(t, 50, int(occupEntity.WorkerPoolCapacity))
				require.Equal(t, 10, int(occupEntity.RunningWorkers))
			},
		},
		{
			"update occupancy",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := occupancies[0].PoolID
				err := occupancyRepo.UpdateWorkerPoolOccupancy(poolID, 10)
				require.NoError(t, err)
				occupEntity, err := occupancyRepo.FindWorkerPoolOccupancyByID(poolID)
				require.NoError(t, err)
				require.Equal(t, 10, int(occupEntity.RunningWorkers))
			},
		},
		{
			"create or update occupancy: create a new one",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := uuid.NewString()
				created, err := occupancyRepo.CreateOrUpdateWorkerPoolOccupancy(poolID, "dummy", 10, 50)
				require.NoError(t, err)
				require.Equal(t, true, created)
				occupEntity, err := occupancyRepo.FindWorkerPoolOccupancyByID(poolID)
				require.NoError(t, err)
				require.Equal(t, 10, int(occupEntity.RunningWorkers))
				require.Equal(t, poolID, occupEntity.WorkerPoolID)
				require.Equal(t, 50, int(occupEntity.WorkerPoolCapacity))
			},
		},
		{
			"create or update occupancy: update an existing one with correct name and poolSize",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := occupancies[1].PoolID
				created, err := occupancyRepo.CreateOrUpdateWorkerPoolOccupancy(poolID, "cp2", 10, 100)
				require.NoError(t, err)
				require.Equal(t, false, created)
				occupEntity, err := occupancyRepo.FindWorkerPoolOccupancyByID(poolID)
				require.NoError(t, err)
				require.Equal(t, 10, int(occupEntity.RunningWorkers))
				require.Equal(t, poolID, occupEntity.WorkerPoolID)
				require.Equal(t, 100, int(occupEntity.WorkerPoolCapacity))
			},
		},
		{
			"create or update occupancy: update an existing one with incorrect name",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := occupancies[1].PoolID
				created, err := occupancyRepo.CreateOrUpdateWorkerPoolOccupancy(poolID, "dummy", 10, 100)
				require.Error(t, err)
				require.Equal(t, false, created)
				occupEntity, err := occupancyRepo.FindWorkerPoolOccupancyByID(poolID)
				require.NoError(t, err)
				require.Equal(t, 0, int(occupEntity.RunningWorkers))
				require.Equal(t, 100, int(occupEntity.WorkerPoolCapacity))
			},
		},
		{
			"create or update occupancy: update an existing one with incorrect poolSize",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := occupancies[1].PoolID
				created, err := occupancyRepo.CreateOrUpdateWorkerPoolOccupancy(poolID, "cp2", 10, 50)
				require.Error(t, err)
				require.Equal(t, false, created)
				occupEntity, err := occupancyRepo.FindWorkerPoolOccupancyByID(poolID)
				require.NoError(t, err)
				require.Equal(t, 0, int(occupEntity.RunningWorkers))
				require.Equal(t, 100, int(occupEntity.WorkerPoolCapacity))
			},
		},
		{
			"get components that registered their occupancy",
			func(t *testing.T, occupancyRepo Repository) {
				componentList, err := occupancyRepo.GetComponentList()
				require.NoError(t, err)
				expectedComponents := []string{"cp1", "cp2", "cp3"}
				require.ElementsMatch(t, expectedComponents, componentList)
			},
		},
		{
			"get componentIDs for components that registered their occupancy",
			func(t *testing.T, occupancyRepo Repository) {
				componentIDs, err := occupancyRepo.GetComponentIDs()
				require.NoError(t, err)
				expectedComponentIDs := []string{"1", "2", "3"}
				require.ElementsMatch(t, expectedComponentIDs, componentIDs)
			},
		},
		{
			"get mean occupancy that is running many worker pools",
			func(t *testing.T, occupancyRepo Repository) {

				component := occupancies[0].Component
				firstPoolID := occupancies[0].PoolID
				err := occupancyRepo.UpdateWorkerPoolOccupancy(firstPoolID, 40)
				require.NoError(t, err)
				secondPoolID := "4"
				_, err = occupancyRepo.CreateWorkerPoolOccupancy(secondPoolID, component, 0, 50)
				require.NoError(t, err)
				err = occupancyRepo.UpdateWorkerPoolOccupancy(secondPoolID, 10)
				require.NoError(t, err)
				meanOccupancy, err := occupancyRepo.GetMeanWorkerPoolOccupancyByComponent(component)
				require.NoError(t, err)
				require.Equal(t, 50.0, meanOccupancy)
			},
		},
	}
	occupancyRepos := newPersistentAndInmemoryRepositories(t)
	for _, occupancyRepo := range occupancyRepos {
		for _, testCase := range testCases {
			unitTestSetup(t, occupancyRepo, occupancies)
			t.Run(testCase.name, newTestFct(testCase, occupancyRepo))
			testCleanUp(t, occupancyRepo)
		}

	}

}

func unitTestSetup(t *testing.T, occupRepo Repository, occupancies []*Occupancy) {
	for _, occupancy := range occupancies {
		_, err := occupRepo.CreateWorkerPoolOccupancy(occupancy.PoolID, occupancy.Component, 0, occupancy.Capacity)
		require.NoError(t, err)
	}
}

func newTestFct(testCase testCase, repo Repository) func(t *testing.T) {
	return func(t *testing.T) {
		t.Log("Executing test case")
		testCase.testFct(t, repo)
	}
}

func testCleanUp(t *testing.T, occupRepo Repository) {
	componentIDs, err := occupRepo.GetComponentIDs()
	require.NoError(t, err)
	_, err = occupRepo.RemoveWorkerPoolOccupancies(componentIDs)
	require.NoError(t, err)
}

func dbConnection(t *testing.T) db.Connection {
	mu.Lock()
	defer mu.Unlock()
	if dbConn == nil {
		dbConn = db.NewTestConnection(t)
	}
	return dbConn
}

func newPersistentAndInmemoryRepositories(t *testing.T) []Repository {
	persistentOccupancyRepository, err := NewPersistentOccupancyRepository(dbConnection(t), true)
	require.NoError(t, err)
	inmemoryOccupancyRepository := NewInMemoryOccupancyRepository()
	return []Repository{persistentOccupancyRepository, inmemoryOccupancyRepository}
}
