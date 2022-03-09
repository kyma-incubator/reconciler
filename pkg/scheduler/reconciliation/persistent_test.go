package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func prepareTest(t *testing.T, schedulingIDsCount int) (Repository, []string) {
	schedulingIDs := make([]string, 0, schedulingIDsCount)
	//create mock database connection
	dbConn := db.NewTestConnection(t)
	reconRepo, _ := NewPersistedReconciliationRepository(dbConn, true)

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster(s) to inventory
	for i := 0; i < schedulingIDsCount; i++ {
		clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "1", 1, false, test.OneComponentDummy))
		require.NoError(t, err)

		//trigger reconciliation for cluster
		reconEntity, _ := reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		schedulingIDs = append(schedulingIDs, reconEntity.SchedulingID)
	}
	return reconRepo, schedulingIDs
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
			reconRepo, schedulingIDsCount := prepareTest(t, testCase.reconciliations)

			if err := reconRepo.RemoveReconciliations(schedulingIDsCount); (err != nil) != testCase.wantErr {
				t.Errorf("RemoveschedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// clean up
			reconciliations, err := reconRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(reconciliations))
		})
	}
}
