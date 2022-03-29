package reconciliation

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

type testEntities struct {
	persistenceRepo          Repository
	inMemoryRepo             Repository
	persistenceSchedulingIDs []string
	inMemorySchedulingIDs    []string
	runtimeIDs               []string
	teardownFn               func()
}

func dbTestConnection(t *testing.T) db.Connection {
	mu.Lock()
	defer mu.Unlock()
	if dbConn == nil {
		dbConn = db.NewTestConnection(t)
	}
	return dbConn
}

func prepareTest(t *testing.T, count int) testEntities {
	//create mock database connection
	dbConn = dbTestConnection(t)

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
			require.NoError(t, inMemoryRepo.RemoveReconciliationByRuntimeID(runtimeID))
			require.NoError(t, inventory.Delete(runtimeID))
		}
	}

	return testEntities{persistenceRepo, inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs, runtimeIDs, teardownFn}
}

func cleanup(teardownFn func()) {
	teardownFn()
}

func TestPersistentReconciliationRepository_RemoveReconciliationsBeforeDeadline(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple reconciliations",
			wantErr:         false,
			reconciliations: 101,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := prepareTest(t, testCase.reconciliations)
			timeTo := time.Now().UTC()
			for _, runtimeID := range testEntities.runtimeIDs {
				if err := testEntities.persistenceRepo.RemoveReconciliationsBeforeDeadline(runtimeID, "nonExistentToMockDeletion", timeTo); (err != nil) != testCase.wantErr {
					t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
				}
				if err := testEntities.inMemoryRepo.RemoveReconciliationsBeforeDeadline(runtimeID, "nonExistentToMockDeletion", timeTo); (err != nil) != testCase.wantErr {
					t.Errorf("InMemory RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
				}
			}

			// check - also ensures clean up
			persistenceReconciliations, err := testEntities.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			cleanup(testEntities.teardownFn)
		})
	}
}

func TestPersistentReconciliationRepository_GetRuntimeIDs(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple reconciliations",
			wantErr:         false,
			reconciliations: 11,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := prepareTest(t, testCase.reconciliations)
			persistentRepoRuntimeIDs, err := testEntities.persistenceRepo.GetRuntimeIDs()
			require.NoError(t, err)
			inmemoryRepoRuntimeIDs, err := testEntities.inMemoryRepo.GetRuntimeIDs()
			require.NoError(t, err)
			sort.Strings(testEntities.runtimeIDs)
			sort.Strings(persistentRepoRuntimeIDs)
			sort.Strings(inmemoryRepoRuntimeIDs)
			require.True(t, reflect.DeepEqual(testEntities.runtimeIDs, persistentRepoRuntimeIDs))
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(testEntities.runtimeIDs, inmemoryRepoRuntimeIDs))
			cleanup(testEntities.teardownFn)
		})
	}
}

func TestPersistentReconciliationRepository_RemoveReconciliationsBySchedulingID(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple reconciliations less than 200 (1 block)",
			wantErr:         false,
			reconciliations: 69,
		},
		{
			name:            "with multiple reconciliations more than 200 (3 blocks)",
			wantErr:         false,
			reconciliations: 409,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := prepareTest(t, testCase.reconciliations)

			if err := testEntities.persistenceRepo.RemoveReconciliationsBySchedulingID(testEntities.persistenceSchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := testEntities.inMemoryRepo.RemoveReconciliationsBySchedulingID(testEntities.inMemorySchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := testEntities.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			cleanup(testEntities.teardownFn)
		})
	}
}

func TestPersistentReconciliationRepository_RemoveReconciliationByRuntimeID(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with one runtime ID",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with no runtime ID",
			wantErr:         false,
			reconciliations: 0,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := prepareTest(t, testCase.reconciliations)
			var runtimeID string
			if testCase.reconciliations > 0 {
				runtimeID = testEntities.runtimeIDs[0]
			}
			if err := testEntities.persistenceRepo.RemoveReconciliationByRuntimeID(runtimeID); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveReconciliationByRuntimeID() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := testEntities.inMemoryRepo.RemoveReconciliationByRuntimeID(runtimeID); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveReconciliationByRuntimeID() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := testEntities.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			cleanup(testEntities.teardownFn)
		})
	}
}

func TestPersistentReconciliationRepository_RemoveReconciliationBySchedulingID(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := prepareTest(t, testCase.reconciliations)
			var schedulingIDPersistent, schedulingIDInMemory string
			if testCase.reconciliations > 0 {
				schedulingIDPersistent = testEntities.persistenceSchedulingIDs[0]
				schedulingIDInMemory = testEntities.inMemorySchedulingIDs[0]
			}
			if err := testEntities.persistenceRepo.RemoveReconciliationBySchedulingID(schedulingIDPersistent); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveReconciliationBySchedulingID() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := testEntities.inMemoryRepo.RemoveReconciliationBySchedulingID(schedulingIDInMemory); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveReconciliationBySchedulingID() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := testEntities.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			cleanup(testEntities.teardownFn)
		})
	}
}

func Test_splitStringSlice(t *testing.T) {
	type args struct {
		slice     []string
		blockSize int
	}
	tests := []struct {
		name string
		args args
		want [][]string
	}{
		{
			name: "when block size is more than max int32",
			args: args{
				slice:     []string{"item1", "item2", "item3"},
				blockSize: math.MaxInt32,
			},
			want: [][]string{{"item1", "item2", "item3"}},
		},
		{
			name: "when a slice of 9 items should be split into blocks of 3",
			args: args{
				slice:     []string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9"},
				blockSize: 3,
			},
			want: [][]string{{"item1", "item2", "item3"}, {"item4", "item5", "item6"}, {"item7", "item8", "item9"}},
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			got := splitStringSlice(testCase.args.slice, testCase.args.blockSize)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("splitStringSlice() got = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestPersistentReconciliationRepository_RemoveReconciliationsForObsoleteStatus(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		want     int
		wantErr  bool
		obsolete bool
		deleted  bool
	}{
		{
			name:     "when reconciliations refer to obsolete cluster config statuses",
			want:     5,
			count:    5,
			deleted:  true,
			obsolete: true,
		},
		{
			name:     "when reconciliations refer to older non-deleted cluster config statuses",
			want:     0,
			count:    5,
			obsolete: true,
			deleted:  false,
		},
		{
			name:     "when reconciliations refer to current deleted cluster config statuses",
			want:     0,
			count:    5,
			obsolete: false,
			deleted:  true,
		},
		{
			name:  "when no reconciliations are present",
			want:  0,
			count: 0,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(tt.name, func(t *testing.T) {
			testEntities := prepareTest(t, testCase.count)
			statusEntity := &model.ClusterStatusEntity{}
			clusterEntity := &model.ClusterEntity{}

			if testCase.deleted {
				for _, runtimeID := range testEntities.runtimeIDs {
					updateSQLTemplate := "UPDATE %s SET %s=$1 WHERE %s=$2"
					_, err := dbConn.Exec(fmt.Sprintf(updateSQLTemplate, statusEntity.Table(), "deleted", "runtime_id"), "TRUE", runtimeID)
					require.NoError(t, err)
					_, err = dbConn.Exec(fmt.Sprintf(updateSQLTemplate, clusterEntity.Table(), "deleted", "runtime_id"), "TRUE", runtimeID)
					require.NoError(t, err)
				}
			}

			deadline := time.Now().UTC()
			if !testCase.obsolete {
				deadline = deadline.Add(-time.Hour)
			}
			got, err := testEntities.persistenceRepo.RemoveReconciliationsForObsoleteStatus(deadline)
			if (err != nil) != testCase.wantErr {
				t.Errorf("RemoveReconciliationsForObsoleteStatus() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if got != testCase.want {
				t.Errorf("RemoveReconciliationsForObsoleteStatus() got = %v, want %v", got, testCase.want)
			}
			cleanup(testEntities.teardownFn)
		})
	}
}
