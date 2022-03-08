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

func prepareTest(t *testing.T, reconCount int) (Repository, []*model.ReconciliationEntity) {
	reconEntities := make([]*model.ReconciliationEntity, 0, reconCount)
	//create mock database connection
	dbConn := db.NewTestConnection(t)
	reconRepo, _ := NewPersistedReconciliationRepository(dbConn, true)

	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster(s) to inventory
	for i := 0; i < reconCount; i++ {
		clusterState, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "1", 1, false, test.OneComponentDummy))
		require.NoError(t, err)

		//trigger reconciliation for cluster
		reconEntity, _ := reconRepo.CreateReconciliation(clusterState, &model.ReconciliationSequenceConfig{})
		reconEntities = append(reconEntities, reconEntity)
	}
	return reconRepo, reconEntities
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
			name:            "with multiple scheduling IDs",
			wantErr:         false,
			reconciliations: 150,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconRepo, reconEntities := prepareTest(t, tt.reconciliations)

			if err := reconRepo.RemoveReconciliations(reconEntities); (err != nil) != tt.wantErr {
				t.Errorf("RemoveschedulingIds() error = %v, wantErr %v", err, tt.wantErr)
			}

			reconciliations, err := reconRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(reconciliations))
		})
	}
}
