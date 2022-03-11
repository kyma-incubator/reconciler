package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"math"
	"reflect"
	"testing"
	"time"
)

func dbTestConnection(t *testing.T) db.Connection {
	mu.Lock()
	defer mu.Unlock()
	if dbConn == nil {
		dbConn = db.NewTestConnection(t)
	}
	return dbConn
}

func prepareTest(t *testing.T, schedulingIDsCount int) (Repository, Repository, []string, []string) {
	//create mock database connection
	dbConn = dbTestConnection(t)

	persistenceSchedulingIDs := make([]string, 0, schedulingIDsCount)
	inMemorySchedulingIDs := make([]string, 0, schedulingIDsCount)

	persistenceRepo, err := NewPersistedReconciliationRepository(dbConn, true)
	require.NoError(t, err)
	inMemoryRepo := NewInMemoryReconciliationRepository()

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	var runtimeIDs []string
	for i := 0; i < schedulingIDsCount; i++ {
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

	return persistenceRepo, inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs
}

func TestPersistentReconciliationRepository_RemoveSchedulingIds(t *testing.T) {
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
			persistenceRepo, inMemoryRepo, persistenceSchedulingIDs, inMemorySchedulingIDs := prepareTest(t, testCase.reconciliations)

			if err := persistenceRepo.RemoveReconciliations(persistenceSchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := inMemoryRepo.RemoveReconciliations(inMemorySchedulingIDs); (err != nil) != testCase.wantErr {
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

func Test_splitStringSlice(t *testing.T) {
	type args struct {
		slice     []string
		blockSize int
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "when block size is more than max int32",
			args: args{
				slice:     []string{"item1", "item2", "item3"},
				blockSize: math.MaxInt32,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "when a slice of 9 items should be split into blocks of 3",
			args: args{
				slice:     []string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9"},
				blockSize: 3,
			},
			want:    [][]string{{"item1", "item2", "item3"}, {"item4", "item5", "item6"}, {"item7", "item8", "item9"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			got, err := splitStringSlice(testCase.args.slice, testCase.args.blockSize)
			if (err != nil) != testCase.wantErr {
				t.Errorf("splitStringSlice() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("splitStringSlice() got = %v, want %v", got, testCase.want)
			}
		})
	}
}
