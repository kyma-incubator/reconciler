package reconciliation

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/stretchr/testify/require"
	"testing"
)

type testCase struct {
	name    string
	testFct func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity
}

var (
	stateMock1 = &cluster.State{
		Cluster: &model.ClusterEntity{
			Cluster: "testCluster1",
		},
		Configuration: &model.ClusterConfigurationEntity{
			Contract: 1,
			Version:  123,
			Components: `[
				{
					"component": "comp1",
					"namespace": "kyma-system",
					"configuration": [
						{
							"key": "limitRange.default.memory",
							"value": "256Mi"
						},
						{
							"key": "limitRange.max.memory",
							"value": "8Gi"
						}
					]
				},
				{
					"component": "comp2",
					"namespace": "istio-system",
					"configuration": []
				},
				{
					"component": "comp3",
					"namespace": "kyma-system",
					"configuration": []
				}
			]`,
		},
		Status: &model.ClusterStatusEntity{},
	}
	stateMock2 = &cluster.State{
		Cluster: &model.ClusterEntity{
			Cluster: "testCluster2",
		},
		Configuration: &model.ClusterConfigurationEntity{
			Contract: 1,
			Version:  123,
			Components: `[
				{
					"component": "comp4",
					"namespace": "kyma-system",
					"configuration": []
				}
			]`,
		},
		Status: &model.ClusterStatusEntity{},
	}
)

func TestReconciliationFindProcessableOps(t *testing.T) {
	ops := []*model.OperationEntity{
		{
			Priority:      1,
			SchedulingID:  "1",
			CorrelationID: "1.1",
			ClusterConfig: 0,
			Component:     "1a",
			State:         model.OperationStateNew,
		},
		{
			Priority:      2,
			SchedulingID:  "1",
			CorrelationID: "1.2",
			ClusterConfig: 0,
			Component:     "2a",
			State:         model.OperationStateNew,
		},
		{
			Priority:      3,
			SchedulingID:  "1",
			CorrelationID: "1.3.1",
			ClusterConfig: 0,
			Component:     "3a",
			State:         model.OperationStateNew,
		},
		{
			Priority:      3,
			SchedulingID:  "1",
			CorrelationID: "1.3.2",
			ClusterConfig: 0,
			Component:     "4a",
			State:         model.OperationStateNew,
		},
		{
			Priority:      1,
			SchedulingID:  "2",
			CorrelationID: "2.1",
			ClusterConfig: 0,
			Component:     "1b",
			State:         model.OperationStateNew,
		},
		{
			Priority:      2,
			SchedulingID:  "2",
			CorrelationID: "2.2",
			ClusterConfig: 0,
			Component:     "2b",
			State:         model.OperationStateNew,
		},
	}

	testCases := map[string]func(t *testing.T){
		"Find prio1": func(t *testing.T) {
			opsGot := findProcessableOperations(ops)
			require.Len(t, opsGot, 2)
			require.ElementsMatch(t, []*model.OperationEntity{ops[0], ops[4]}, opsGot)
		},
		"Find prio1 with failure": func(t *testing.T) {
			ops[0].State = model.OperationStateFailed
			opsGot := findProcessableOperations(ops)
			require.Len(t, opsGot, 2)
			require.ElementsMatch(t, []*model.OperationEntity{ops[0], ops[4]}, opsGot)
		},
		"Find prio2": func(t *testing.T) {
			ops[0].State = model.OperationStateDone
			ops[4].State = model.OperationStateDone
			opsGot := findProcessableOperations(ops)
			require.Len(t, opsGot, 2)
			require.ElementsMatch(t, []*model.OperationEntity{ops[1], ops[5]}, opsGot)
		},
		"Find prio2 with in progress": func(t *testing.T) {
			ops[0].State = model.OperationStateDone
			ops[1].State = model.OperationStateInProgress
			ops[4].State = model.OperationStateDone
			ops[5].State = model.OperationStateInProgress
			opsGot := findProcessableOperations(ops)
			require.Empty(t, opsGot)
		},
		"Find prio3": func(t *testing.T) {
			ops[0].State = model.OperationStateDone
			ops[1].State = model.OperationStateDone
			ops[4].State = model.OperationStateDone
			opsGot := findProcessableOperations(ops)
			require.Len(t, opsGot, 3)
			require.ElementsMatch(t, []*model.OperationEntity{ops[2], ops[3], ops[5]}, opsGot)
		},
		"Find with error at prio 1": func(t *testing.T) {
			ops[0].State = model.OperationStateError
			ops[4].State = model.OperationStateError
			opsGot := findProcessableOperations(ops)
			require.Empty(t, opsGot)
		},
		"Find with error at prio 2": func(t *testing.T) {
			ops[0].State = model.OperationStateDone
			ops[1].State = model.OperationStateError
			ops[4].State = model.OperationStateDone
			ops[5].State = model.OperationStateError
			opsGot := findProcessableOperations(ops)
			require.Empty(t, opsGot)
		},
		"Find with error at prio 3": func(t *testing.T) {
			ops[0].State = model.OperationStateDone
			ops[1].State = model.OperationStateDone
			ops[2].State = model.OperationStateError
			opsGot := findProcessableOperations(ops)
			require.ElementsMatch(t, []*model.OperationEntity{ops[4]}, opsGot)
		},
	}

	for name, testCaseFct := range testCases {
		t.Run(name, testCaseFct)
		resetOperationState(ops)
	}

}

func resetOperationState(ops []*model.OperationEntity) {
	for _, op := range ops {
		op.State = model.OperationStateNew
	}
}

func TestReconciliationRepository(t *testing.T) {
	var testCases = []testCase{
		{
			name: "Create reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)

				require.NoError(t, err)
				require.NotEmpty(t, reconEntity.SchedulingID)
				require.Equal(t, stateMock1.Cluster.Cluster, reconEntity.Lock)
				require.Equal(t, stateMock1.Cluster.Cluster, reconEntity.Cluster)
				require.Equal(t, stateMock1.Configuration.Version, reconEntity.ClusterConfig)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get existing reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)
				reconGot, err := reconRepo.GetReconciliation(reconEntity.SchedulingID)
				require.NoError(t, err)
				require.Equal(t, reconEntity.SchedulingID, reconGot.SchedulingID)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get non-existing reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				_, err := reconRepo.GetReconciliation("dont exist")
				require.Error(t, err)
				require.True(t, repository.IsNotFoundError(err))

				return nil
			},
		},
		{
			name: "Create duplicate reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)

				_, err = reconRepo.CreateReconciliation(stateMock1, nil)
				require.Error(t, err)
				require.True(t, IsDuplicateClusterReconciliationError(err))

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Finish reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)

				err = reconRepo.FinishReconciliation(reconEntity.SchedulingID, &model.ClusterStatusEntity{
					ID: 9999,
				})
				require.NoError(t, err)

				//finish a non-running reconciliation is not allowed
				err = reconRepo.FinishReconciliation(reconEntity.SchedulingID, &model.ClusterStatusEntity{
					ID: 9999,
				})
				require.Error(t, err)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get reconciliations with and without filter",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity1, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)
				reconEntity2, err := reconRepo.CreateReconciliation(stateMock2, nil)
				require.NoError(t, err)

				all, err := reconRepo.GetReconciliations(nil)
				require.Len(t, all, 2)
				require.NoError(t, err)

				only2, err := reconRepo.GetReconciliations(&WithSchedulingID{reconEntity2.SchedulingID})
				require.NoError(t, err)
				require.Len(t, only2, 1)
				require.Equal(t, reconEntity2.SchedulingID, only2[0].SchedulingID)

				only1, err := reconRepo.GetReconciliations(&WithCluster{reconEntity1.Cluster})
				require.NoError(t, err)
				require.Len(t, only1, 1)
				require.Equal(t, reconEntity1.SchedulingID, only1[0].SchedulingID)

				err = reconRepo.FinishReconciliation(reconEntity1.SchedulingID, &model.ClusterStatusEntity{
					ID: 9999,
				})
				require.NoError(t, err)

				recon, err := reconRepo.GetReconciliations(&CurrentlyReconciling{})
				require.NoError(t, err)
				require.Len(t, recon, 1)
				require.Equal(t, reconEntity2.SchedulingID, recon[0].SchedulingID)

				return []*model.ReconciliationEntity{reconEntity1, reconEntity2}
			},
		},
		{
			name: "Remove reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)

				err = reconRepo.RemoveReconciliation(reconEntity.SchedulingID)
				require.NoError(t, err)

				//try to delete non-exiting reconciliation (no error expected)
				err = reconRepo.RemoveReconciliation("123-456")
				require.NoError(t, err)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get operations",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, []string{"comp3"})
				require.NoError(t, err)

				opsEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				require.Len(t, opsEntities, 4)

				//verify priorities
				for _, opEntity := range opsEntities {
					switch opEntity.Component {
					case "CRDs":
						require.Equal(t, int64(1), opEntity.Priority)
					case "comp3":
						require.Equal(t, int64(2), opEntity.Priority)
					default:
						require.Equal(t, int64(3), opEntity.Priority)
					}
				}

				op, err := reconRepo.GetOperation(reconEntity.SchedulingID, opsEntities[1].CorrelationID)
				require.NoError(t, err)
				require.Equal(t, opsEntities[1], op)

				//ensure also operations are dropped
				err = reconRepo.RemoveReconciliation(reconEntity.SchedulingID)
				require.NoError(t, err)

				opsEntities, err = reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				require.Empty(t, opsEntities)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get operations with filter",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)

				opsEntitiesAll, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				require.Len(t, opsEntitiesAll, 4)

				opsEntitiesNew, err := reconRepo.GetOperations(reconEntity.SchedulingID, model.OperationStateNew)
				require.NoError(t, err)
				require.Len(t, opsEntitiesNew, 4)

				err = reconRepo.UpdateOperationState(opsEntitiesAll[0].SchedulingID, opsEntitiesAll[0].CorrelationID,
					model.OperationStateError, "err")
				require.NoError(t, err)
				opsEntitiesErr, err := reconRepo.GetOperations(reconEntity.SchedulingID, model.OperationStateError)
				require.NoError(t, err)
				require.Len(t, opsEntitiesErr, 1)
				require.Equal(t, opsEntitiesAll[0].CorrelationID, opsEntitiesErr[0].CorrelationID)

				err = reconRepo.UpdateOperationState(opsEntitiesAll[1].SchedulingID, opsEntitiesAll[1].CorrelationID,
					model.OperationStateFailed, "err")
				require.NoError(t, err)
				opsEntitiesFailed, err := reconRepo.GetOperations(reconEntity.SchedulingID, model.OperationStateFailed)
				require.NoError(t, err)
				require.Len(t, opsEntitiesFailed, 1)
				require.Equal(t, opsEntitiesAll[1].CorrelationID, opsEntitiesFailed[0].CorrelationID)

				err = reconRepo.UpdateOperationState(opsEntitiesAll[2].SchedulingID, opsEntitiesAll[2].CorrelationID,
					model.OperationStateDone)
				require.NoError(t, err)
				err = reconRepo.UpdateOperationState(opsEntitiesAll[3].SchedulingID, opsEntitiesAll[3].CorrelationID,
					model.OperationStateDone)
				require.NoError(t, err)
				opsEntitiesDone, err := reconRepo.GetOperations(reconEntity.SchedulingID, model.OperationStateDone)
				require.NoError(t, err)
				require.Len(t, opsEntitiesDone, 2)
				require.ElementsMatch(t, []string{
					opsEntitiesAll[2].CorrelationID,
					opsEntitiesAll[3].CorrelationID,
				}, []string{
					opsEntitiesDone[0].CorrelationID,
					opsEntitiesDone[1].CorrelationID,
				})

				//no operation should be in state NEW anymore
				opsEntitiesNew, err = reconRepo.GetOperations(reconEntity.SchedulingID, model.OperationStateNew)
				require.NoError(t, err)
				require.Len(t, opsEntitiesNew, 0)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get processable operations using 1 reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, []string{"comp1"})
				require.NoError(t, err)

				//get existing operations
				opsEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				require.Len(t, opsEntities, 4)

				//only the operation with prio 1 has to be returned
				opsEntitiesPrio1, err := reconRepo.GetProcessableOperations()
				require.NoError(t, err)
				require.Len(t, opsEntitiesPrio1, 1)
				require.ElementsMatch(t, findOperationsByPrio(opsEntities, 1), opsEntitiesPrio1)

				//mark processable prio 1 operation as done
				for _, op := range opsEntitiesPrio1 {
					require.NoError(t, reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID,
						model.OperationStateDone))
				}

				opsEntitiesPrio2, err := reconRepo.GetProcessableOperations()
				require.NoError(t, err)
				require.Len(t, opsEntitiesPrio2, 1)
				require.ElementsMatch(t, findOperationsByPrio(opsEntities, 2), opsEntitiesPrio2)

				//mark processable prio 2 operation to be in error state
				for _, op := range opsEntitiesPrio2 {
					require.NoError(t, reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID,
						model.OperationStateError, "I failed"))
				}

				//one of the previous operations is in error state: no further operations have to be processed
				opsEntitiesPrio, err := reconRepo.GetProcessableOperations()
				require.NoError(t, err)
				require.Empty(t, opsEntitiesPrio)

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
		{
			name: "Get processable operations using 2 reconciliation",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity1, err := reconRepo.CreateReconciliation(stateMock1, []string{"comp1"})
				require.NoError(t, err)
				reconEntity2, err := reconRepo.CreateReconciliation(stateMock2, nil)
				require.NoError(t, err)

				//get existing operations
				opsEntities1, err := reconRepo.GetOperations(reconEntity1.SchedulingID)
				require.NoError(t, err)
				require.Len(t, opsEntities1, 4)
				opsEntities2, err := reconRepo.GetOperations(reconEntity2.SchedulingID)
				require.NoError(t, err)
				require.Len(t, opsEntities2, 2)

				//only the operation with prio 1 has to be returned
				opsEntitiesPrio1, err := reconRepo.GetProcessableOperations()
				var expectedOpsPrio1 []*model.OperationEntity
				expectedOpsPrio1 = append(expectedOpsPrio1, findOperationsByPrio(opsEntities1, 1)...)
				expectedOpsPrio1 = append(expectedOpsPrio1, findOperationsByPrio(opsEntities2, 1)...)
				require.NoError(t, err)
				require.Len(t, opsEntitiesPrio1, 2)
				require.ElementsMatch(t, expectedOpsPrio1, opsEntitiesPrio1)

				//mark processable prio 1 operation as done
				for _, op := range opsEntitiesPrio1 {
					require.NoError(t, reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID,
						model.OperationStateDone))
				}

				opsEntitiesPrio2, err := reconRepo.GetProcessableOperations()
				var expectedOpsPrio2 []*model.OperationEntity
				expectedOpsPrio2 = append(expectedOpsPrio2, findOperationsByPrio(opsEntities1, 2)...)
				expectedOpsPrio2 = append(expectedOpsPrio2, findOperationsByPrio(opsEntities2, 2)...)
				require.NoError(t, err)
				require.Len(t, opsEntitiesPrio2, 2)
				require.ElementsMatch(t, expectedOpsPrio2, opsEntitiesPrio2)

				//mark processable prio 2 operation to be in error state
				for _, op := range opsEntitiesPrio2 {
					require.NoError(t, reconRepo.UpdateOperationState(op.SchedulingID, op.CorrelationID,
						model.OperationStateError, "I failed"))
				}

				//one of the previous operations is in error state: no further operations have to be processed
				opsEntitiesPrio, err := reconRepo.GetProcessableOperations()
				require.NoError(t, err)
				require.Empty(t, opsEntitiesPrio)

				return []*model.ReconciliationEntity{reconEntity1, reconEntity2}
			},
		},
		{
			name: "Get reconciling operations",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity1, err := reconRepo.CreateReconciliation(stateMock1, []string{"comp1"})
				require.NoError(t, err)
				reconEntity2, err := reconRepo.CreateReconciliation(stateMock2, nil)
				require.NoError(t, err)

				//get existing operations
				opsRecon, err := reconRepo.GetReconcilingOperations()
				require.NoError(t, err)
				require.Len(t, opsRecon, 6)

				return []*model.ReconciliationEntity{reconEntity1, reconEntity2}
			},
		},
		{
			name: "Set operation states",
			testFct: func(t *testing.T, reconRepo Repository) []*model.ReconciliationEntity {
				reconEntity, err := reconRepo.CreateReconciliation(stateMock1, nil)
				require.NoError(t, err)

				opsEntities, err := reconRepo.GetOperations(reconEntity.SchedulingID)
				require.NoError(t, err)
				require.Len(t, opsEntities, 4)

				sID := opsEntities[0].SchedulingID
				cID := opsEntities[0].CorrelationID

				require.NoError(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateInProgress))
				op, _ := reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateInProgress)

				require.NoError(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateClientError, "client error reason"))
				op, _ = reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateClientError, "client error reason")

				require.NoError(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateInProgress))
				op, _ = reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateInProgress)

				require.NoError(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateFailed, "operation failed reason"))
				op, _ = reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateFailed, "operation failed reason")

				require.NoError(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateInProgress))
				op, _ = reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateInProgress)

				require.NoError(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateError, "operation error reason"))
				op, _ = reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateError, "operation error reason")

				//expect an error because operation is in final state
				require.Error(t, reconRepo.UpdateOperationState(sID, cID, model.OperationStateInProgress))
				op, _ = reconRepo.GetOperation(sID, cID)
				verifyOperationstateMock(t, op, model.OperationStateError, "operation error reason")

				return []*model.ReconciliationEntity{reconEntity}
			},
		},
	}

	repos := map[string]Repository{
		"persistent": newPersistentRepository(t),
		"in-memory":  NewInMemoryReconciliationRepository()}

	for _, testCase := range testCases {
		for repoName, repo := range repos {
			t.Run(fmt.Sprintf("%s: %s", repoName, testCase.name), func(t *testing.T) {
				reconEntities := testCase.testFct(t, repo)

				//cleanup
				for _, reconEntity := range reconEntities {
					if reconEntity != nil {
						require.NoError(t, repo.RemoveReconciliation(reconEntity.SchedulingID))
					}
				}
			})
		}
	}

}

func verifyOperationstateMock(t *testing.T, op *model.OperationEntity, expectedState model.OperationState, reasons ...string) {
	require.Equal(t, expectedState, op.State)
	reason, err := concatStateReasons(expectedState, reasons)
	require.NoError(t, err)
	require.Equal(t, reason, op.Reason)
}

func newPersistentRepository(t *testing.T) Repository {
	reconRepo, err := NewPersistedReconciliationRepository(db.NewTestConnection(t), true)
	require.NoError(t, err)

	return reconRepo
}

func findOperationsByPrio(ops []*model.OperationEntity, prio int) []*model.OperationEntity {
	var result []*model.OperationEntity
	for _, op := range ops {
		if op.Priority == int64(prio) {
			result = append(result, op)
		}
	}
	return result
}
