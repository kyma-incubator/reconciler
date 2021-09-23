package reconciliation

import (
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"sort"
)

type Filter interface {
	FilterByQuery(q *db.Select) error
	FilterByInstance(i *model.ReconciliationEntity) *model.ReconciliationEntity //return nil to ignore instance in result
}

type Repository interface {
	CreateReconciliation(state *cluster.State, prerequisites []string) (*model.ReconciliationEntity, error)
	RemoveReconciliation(schedulingID string) error
	GetReconciliation(schedulingID string) (*model.ReconciliationEntity, error)
	GetReconciliations(filter Filter) ([]*model.ReconciliationEntity, error)
	FinishReconciliation(schedulingID string, status *model.ClusterStatusEntity) error
	GetOperations(schedulingID string) ([]*model.OperationEntity, error)
	GetOperation(schedulingID, correlationID string) (*model.OperationEntity, error)
	GetProcessableOperations() ([]*model.OperationEntity, error)
	SetOperationInProgress(schedulingID, correlationID string) error
	SetOperationDone(schedulingID, correlationID string) error
	SetOperationError(schedulingID, correlationID, reason string) error
	SetOperationClientError(schedulingID, correlationID, reason string) error
	SetOperationFailed(schedulingID, correlationID, reason string) error
}

//findProcessableOperations returns all operations in all running reconciliation which are ready to be processed.
//The priority of an operation is considered (1=highest priority, 2-x=lower priorities).
//An operation with a high priority has first to be finished before operations with a lower priority
//are considered as processable.
func findProcessableOperations(ops []*model.OperationEntity) []*model.OperationEntity {
	//group ops per reconciliation and their prio
	groupedByReconAndPrio := make(map[string]map[int64][]*model.OperationEntity) //key1:schedulingID, key2:prio
	for _, op := range ops {
		if _, ok := groupedByReconAndPrio[op.SchedulingID]; !ok {
			groupedByReconAndPrio[op.SchedulingID] = make(map[int64][]*model.OperationEntity)
		}
		samePrioGroup, ok := groupedByReconAndPrio[op.SchedulingID][op.Priority]
		if ok {
			samePrioGroup = append(samePrioGroup, op)
		} else {
			samePrioGroup = []*model.OperationEntity{op}
		}
		groupedByReconAndPrio[op.SchedulingID][op.Priority] = samePrioGroup
	}

	//find per reconciliation the processable ops in a prio-group (searching from highest to lowest prio-group)
	var result []*model.OperationEntity

	for _, opsWithSamePrio := range groupedByReconAndPrio { //iterate of reconciliations
		for _, prio := range prios(opsWithSamePrio) { //iterate over prio-groups
			processable, checkNextGroup := findProcessableOperationsInGroup(opsWithSamePrio[prio])
			if checkNextGroup {
				continue
			}
			result = append(result, processable...)
			break
		}
	}
	return result
}

func prios(opsByPrio map[int64][]*model.OperationEntity) []int64 {
	var prios []int64
	for prio := range opsByPrio {
		prios = append(prios, prio)
	}

	sort.Slice(prios, func(p, q int) bool {
		return prios[p] < prios[q]
	})

	return prios
}

//findProcessableOperationsInGroup returns all operations in the group which are processable.
//The second return value indicates whether the next processing group should be evaluated:
// * true: all operations of the current group were successfully completed and next group shoud be evaluated.
// * false: next group should not be evaluated. This is the case when either the current group
//          is still in progress or >= 1 operations of the current group are in error state.
func findProcessableOperationsInGroup(ops []*model.OperationEntity) ([]*model.OperationEntity, bool) {
	var opsInProgress int
	var processables []*model.OperationEntity
	for _, op := range ops {
		switch op.State {
		case model.OperationStateInProgress:
			opsInProgress++
			continue
		case model.OperationStateDone:
			continue
		case model.OperationStateError:
			return nil, false
		default:
			processables = append(processables, op)
		}
	}
	return processables, opsInProgress == 0 && len(processables) == 0
}
