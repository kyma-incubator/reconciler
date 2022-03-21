package reconciliation

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func dbTestConnection(t *testing.T) db.Connection {
	mu.Lock()
	defer mu.Unlock()
	if dbConn == nil {
		dbConn = db.NewTestConnection(t)
	}
	return dbConn
}

func prepareTest(t *testing.T, count int) (Repository, Repository, []string, []string, []string) {
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
	defer func() {
		for _, runtimeID := range runtimeIDs {
			require.NoError(t, inventory.Delete(runtimeID))
		}
	}()

	return persistenceRepo, inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs, runtimeIDs
}

func TestPersistentReconciliationRepository_RemoveReconciliationsByFilter(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no scheduling IDs",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one scheduling ID",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple scheduling IDs",
			wantErr:         false,
			reconciliations: 101,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			persistenceRepo, inMemoryRepo, _, _, runtimeIDs := prepareTest(t, testCase.reconciliations)

			timeTo := time.Now().UTC()
			timeFrom := time.Now().UTC().Add(-1 * 24 * time.Hour)

			for _, runtimeID := range runtimeIDs {
				timeFromFilter := WithCreationDateAfter{
					Time: timeFrom,
				}
				timeToFilter := WithCreationDateBefore{
					Time: timeTo,
				}
				reconciliationIDFilter := WithRuntimeID{
					RuntimeID: runtimeID,
				}

				filter := FilterMixer{
					Filters: []Filter{&timeFromFilter, &timeToFilter, &reconciliationIDFilter},
				}

				if err := persistenceRepo.RemoveReconciliations(&filter); (err != nil) != testCase.wantErr {
					t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
				}
				if err := inMemoryRepo.RemoveReconciliations(&filter); (err != nil) != testCase.wantErr {
					t.Errorf("InMemory RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
				}
			}

			// check - also ensures clean up
			persistenceReconciliations, err := persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
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
			name:            "with no scheduling IDs",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one scheduling ID",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple scheduling IDs less than 200 (1 block)",
			wantErr:         false,
			reconciliations: 69,
		},
		{
			name:            "with multiple scheduling IDs more than 200 (3 blocks)",
			wantErr:         false,
			reconciliations: 409,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			persistenceRepo, inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs, _ := prepareTest(t, testCase.reconciliations)

			if err := persistenceRepo.RemoveReconciliationsBySchedulingID(persistenceSchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := inMemoryRepo.RemoveReconciliationsBySchedulingID(inMemorySchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
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
			persistenceRepo, inMemoryRepo, _, _, runtimeIDs := prepareTest(t, testCase.reconciliations)
			var runtimeID string
			if testCase.reconciliations > 0 {
				runtimeID = runtimeIDs[0]
			}
			if err := persistenceRepo.RemoveReconciliationByRuntimeID(runtimeID); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveReconciliationByRuntimeID() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := inMemoryRepo.RemoveReconciliationByRuntimeID(runtimeID); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveReconciliationByRuntimeID() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
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
			name:            "with one scheduling ID",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with no scheduling ID",
			wantErr:         false,
			reconciliations: 0,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			persistenceRepo, inMemoryRepo, schedulingIDsPersistent, schedulingIDsInMemory, _ := prepareTest(t, testCase.reconciliations)
			var schedulingIDPersistent, schedulingIDInMemory string
			if testCase.reconciliations > 0 {
				schedulingIDPersistent = schedulingIDsPersistent[0]
				schedulingIDInMemory = schedulingIDsInMemory[0]
			}
			if err := persistenceRepo.RemoveReconciliationBySchedulingID(schedulingIDPersistent); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveReconciliationBySchedulingID() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := inMemoryRepo.RemoveReconciliationBySchedulingID(schedulingIDInMemory); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveReconciliationBySchedulingID() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
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
