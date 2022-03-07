package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb/test"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"testing"
)

var (
	reconciliationEntities []*model.ReconciliationEntity
)

//func TestMain() {
//
//}

func prepare(t *testing.T) Repository {
	//create mock database connection
	dbConn := db.NewTestConnection(t)
	//prepare inventory
	inventory, err := cluster.NewInventory(dbConn, true, cluster.MetricsCollectorMock{})
	require.NoError(t, err)

	//add cluster to inventory
	clusterState1, err := inventory.CreateOrUpdate(1, test.NewCluster(t, "1", 1, false, test.OneComponentDummy))
	clusterState2, err := inventory.CreateOrUpdate(2, test.NewCluster(t, "1", 2, false, test.OneComponentDummy))
	require.NoError(t, err)

	//trigger reconciliation for cluster
	reconRepo, _ := NewPersistedReconciliationRepository(dbConn, true)
	reconciliationEntity1, _ := reconRepo.CreateReconciliation(clusterState1, &model.ReconciliationSequenceConfig{})
	reconciliationEntities = append(reconciliationEntities, reconciliationEntity1)
	reconciliationEntity2, _ := reconRepo.CreateReconciliation(clusterState2, &model.ReconciliationSequenceConfig{})
	reconciliationEntities = append(reconciliationEntities, reconciliationEntity2)
	return reconRepo
}

func TestPersistentReconciliationRepository_RemoveReconciliationEntities(t *testing.T) {
	dbConn = dbConnection(t)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//r := &PersistentReconciliationRepository{
			//	Repository: repository,
			//}
			r := prepare(t)
			if err := r.RemoveReconciliationEntities(reconciliationEntities); (err != nil) != tt.wantErr {
				t.Errorf("RemoveReconciliationEntities() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
