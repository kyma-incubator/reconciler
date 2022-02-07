package occupancy

import (
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/db"
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
			"create occupancy",
			func(t *testing.T, occupancyRepo Repository) {
				poolID := uuid.NewString()
				occupEntity, err := occupancyRepo.CreateWorkerPoolOccupancy(poolID, "component1", 50)
				require.NoError(t, err)
				require.Equal(t, poolID, occupEntity.WorkerPoolID)
				require.Equal(t, 50, int(occupEntity.WorkerPoolCapacity))
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
			"get components that registered their occupancy",
			func(t *testing.T, occupancyRepo Repository) {
				componentList, err := occupancyRepo.GetComponentList()
				require.NoError(t, err)
				expectedComponents := []string{"cp1", "cp2", "cp3"}
				require.ElementsMatch(t, expectedComponents, componentList)
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
				_, err = occupancyRepo.CreateWorkerPoolOccupancy(secondPoolID, component, 50)
				require.NoError(t, err)
				err = occupancyRepo.UpdateWorkerPoolOccupancy(secondPoolID, 10)
				require.NoError(t, err)
				meanOccupancy, err := occupancyRepo.GetMeanWorkerPoolOccupancyByComponent(component)
				require.NoError(t, err)
				require.Equal(t, 50.0, meanOccupancy)
			},
		},
	}

	for _, testCase := range testCases {
		occupancyRepo := newPersistentRepository(t)
		unitTestSetup(t, occupancyRepo, occupancies)
		t.Run(testCase.name, newTestFct(testCase, occupancyRepo))
		testCleanUp(t, occupancyRepo)
	}

}

func unitTestSetup(t *testing.T, occupRepo Repository, occupancies []*Occupancy) {
	for _, occupancy := range occupancies {
		_, err := occupRepo.CreateWorkerPoolOccupancy(occupancy.PoolID, occupancy.Component, occupancy.Capacity)
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
	occupancies, err := occupRepo.GetWorkerPoolOccupancies()
	require.NoError(t, err)
	for _, occupancy := range occupancies {
		err := occupRepo.RemoveWorkerPoolOccupancy(occupancy.WorkerPoolID)
		require.NoError(t, err)
	}
}

func dbConnection(t *testing.T) db.Connection {
	mu.Lock()
	defer mu.Unlock()
	if dbConn == nil {
		dbConn = db.NewTestConnection(t)
	}
	return dbConn
}

func newPersistentRepository(t *testing.T) Repository {
	occupancyRepository, err := NewPersistentOccupancyRepository(dbConnection(t), true)
	require.NoError(t, err)

	return occupancyRepository
}
