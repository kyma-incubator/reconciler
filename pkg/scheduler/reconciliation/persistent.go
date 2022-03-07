package reconciliation

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/cluster"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation/operation"
	"github.com/pkg/errors"
)

type PersistentReconciliationRepository struct {
	*repository.Repository
}

func NewPersistedReconciliationRepository(conn db.Connection, debug bool) (Repository, error) {
	repo, err := repository.NewRepository(conn, debug)
	if err != nil {
		return nil, err
	}
	return &PersistentReconciliationRepository{repo}, nil
}

func (r *PersistentReconciliationRepository) WithTx(tx *db.TxConnection) (Repository, error) {
	return NewPersistedReconciliationRepository(tx, r.Debug)
}

func (r *PersistentReconciliationRepository) CreateReconciliation(state *cluster.State, cfg *model.ReconciliationSequenceConfig) (*model.ReconciliationEntity, error) {
	if len(state.Configuration.Components) == 0 {
		return nil, newEmptyComponentsReconciliationError(state)
	}

	dbOps := func(tx *db.TxConnection) (interface{}, error) {
		reconEntity := &model.ReconciliationEntity{
			Lock:                state.Cluster.RuntimeID,
			RuntimeID:           state.Cluster.RuntimeID,
			ClusterConfig:       state.Configuration.Version,
			ClusterConfigStatus: state.Status.ID,
			SchedulingID:        fmt.Sprintf("%s--%s", state.Cluster.RuntimeID, uuid.NewString()),
			Status:              state.Status.Status,
		}

		//find existing reconciliation for this cluster
		existingReconQ, err := db.NewQuery(tx, reconEntity, r.Logger)
		if err != nil {
			return nil, err
		}
		existingRecon, err := existingReconQ.
			Select().
			Where(map[string]interface{}{
				"RuntimeID": state.Cluster.RuntimeID,
				"Finished":  false,
			}).
			GetOne()
		if err == nil {
			existingReconEntity := existingRecon.(*model.ReconciliationEntity)
			r.Logger.Infof("ReconRepo found existing reconciliation for cluster '%s' (configVersion:%d) "+
				"which was created at '%s': cannot create another one",
				existingReconEntity.RuntimeID, existingReconEntity.ClusterConfig, existingReconEntity.Created)
			return nil, newDuplicateClusterReconciliationError(existingReconEntity)
		}
		if err != sql.ErrNoRows {
			r.Logger.Errorf("ReconRepo failed to check for existing reconciliations entities: %s", err)
			return nil, err
		}

		createReconQ, err := db.NewQuery(tx, reconEntity, r.Logger)
		if err != nil {
			return nil, err
		}
		if err := createReconQ.Insert().Exec(); err != nil {
			r.Logger.Errorf("ReconRepo failed to create new reconciliation entity for runtime '%s': %s",
				state.Cluster.RuntimeID, err)
			return nil, err
		}
		r.Logger.Debugf("ReconRepo created new reconciliation for runtime '%s' with schedulingID '%s'",
			state.Cluster.RuntimeID, reconEntity.SchedulingID)

		opType := model.OperationTypeReconcile
		if state.Status.Status.IsDeletionInProgress() {
			opType = model.OperationTypeDelete
		}

		//iterate over reconciliation sequence and create operations with proper priorities
		var opsList bytes.Buffer

		//get reconciliation sequence
		sequence := state.Configuration.GetReconciliationSequence(cfg)

		for idx, components := range sequence.Queue {
			priority := idx + 1
			for _, component := range components {
				createOpQ, err := db.NewQuery(tx, &model.OperationEntity{
					Priority:      int64(priority),
					SchedulingID:  reconEntity.SchedulingID,
					CorrelationID: fmt.Sprintf("%s--%s", state.Cluster.RuntimeID, uuid.NewString()),
					RuntimeID:     reconEntity.RuntimeID,
					ClusterConfig: reconEntity.ClusterConfig,
					Component:     component.Component,
					State:         model.OperationStateNew,
					Type:          opType,
					RetryID:       uuid.NewString(),
					Updated:       time.Now().UTC(),
				}, r.Logger)
				if err != nil {
					return nil, err
				}

				if err := createOpQ.Insert().Exec(); err != nil {
					r.Logger.Errorf("ReconRepo failed to create operation for component '%s' with priority %d "+
						"(schedulingID:%s/runtimeID:%s): %s",
						component.Component, priority, reconEntity.SchedulingID, state.Cluster.RuntimeID, err)
					return nil, err
				}

				//list created ops in log-msg
				if opsList.Len() > 0 {
					opsList.WriteRune(',')
				}
				opsList.WriteString(fmt.Sprintf("%s@%s[%d]", component.Component, component.Namespace, priority))
			}
		}

		r.Logger.Infof("ReconRepo created reconciliation (schedulingID:%s) for cluster '%s' including following operations: %s",
			reconEntity.SchedulingID, reconEntity.RuntimeID, opsList.String())

		return reconEntity, err
	}
	result, err := db.TransactionResult(r.Conn, dbOps, r.Logger)
	if err != nil {
		return nil, err
	}
	return result.(*model.ReconciliationEntity), nil
}

func (r *PersistentReconciliationRepository) RemoveReconciliation(schedulingID string) error {
	dbOps := func(tx *db.TxConnection) error {
		whereCond := map[string]interface{}{
			"SchedulingID": schedulingID,
		}

		//delete operations
		qDelOps, err := db.NewQuery(tx, &model.OperationEntity{}, r.Logger)
		if err != nil {
			return err
		}
		delOpsCnt, err := qDelOps.Delete().
			Where(whereCond).
			Exec()
		if err != nil {
			return err
		}
		r.Logger.Debugf("ReconRepo deleted %d operations which were assigned to reconciliation with schedulingID '%s'",
			delOpsCnt, schedulingID)

		//delete reconciliation
		qDelRecon, err := db.NewQuery(tx, &model.ReconciliationEntity{}, r.Logger)
		if err != nil {
			return err
		}
		delCnt, err := qDelRecon.Delete().
			Where(whereCond).
			Exec()
		r.Logger.Debugf("Deleted %d reconciliation with schedulingID '%s'", delCnt, schedulingID)
		return err
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) RemoveReconciliationEntities(reconciliationEntities []*model.ReconciliationEntity) error {
	dbOps := func(tx *db.TxConnection) error {
		// format scheduling IDs
		sIdsDuplicate := make(map[string]interface{}, len(reconciliationEntities))
		var buffer bytes.Buffer
		for _, reconciliationEntity := range reconciliationEntities {
			if _, ok := sIdsDuplicate[reconciliationEntity.SchedulingID]; ok {
				continue
			}
			sIdsDuplicate[reconciliationEntity.SchedulingID] = nil

			if buffer.Len() > 0 {
				buffer.WriteRune(',')
			}
			buffer.WriteString(fmt.Sprintf("'%s'", reconciliationEntity.SchedulingID))
		}

		//delete operations
		deleteQuery, err := db.NewQuery(tx, &model.OperationEntity{}, r.Logger)
		if err != nil {
			return err
		}
		deleteQueryCount, err := deleteQuery.Delete().WhereIn("SchedulingID", buffer.String()).Exec()
		if err != nil {
			return err
		}
		r.Logger.Debugf("ReconRepo deleted %d operations which were assigned to reconciliation with schedulingIDs '%s'", deleteQueryCount, buffer.String())
		return nil
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) GetReconciliation(schedulingID string) (*model.ReconciliationEntity, error) {
	q, err := db.NewQuery(r.Conn, &model.ReconciliationEntity{}, r.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"SchedulingID": schedulingID,
	}
	reconEntity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, r.MapError(err, reconEntity, whereCond)
	}
	return reconEntity.(*model.ReconciliationEntity), nil
}

func (r *PersistentReconciliationRepository) FinishReconciliation(schedulingID string, status *model.ClusterStatusEntity) error {
	dbOps := func(tx *db.TxConnection) error {
		//get running reconciliation
		reconEntity, err := r.GetReconciliation(schedulingID)
		if err != nil {
			return err
		}

		//update reconciliation and remove lock
		reconEntity.Lock = fmt.Sprintf("unlock-%s", reconEntity.SchedulingID)
		reconEntity.Finished = true
		reconEntity.ClusterConfigStatus = status.ID
		reconEntity.Status = status.Status
		reconEntity.Updated = time.Now().UTC()
		updReconQ, err := db.NewQuery(tx, reconEntity, r.Logger)
		if err != nil {
			return err
		}
		cnt, err := updReconQ.Update().
			Where(
				map[string]interface{}{
					"SchedulingID": schedulingID,
					"Finished":     false,
				}).
			ExecCount()
		if err != nil {
			return err
		}
		if cnt == 0 {
			return fmt.Errorf("failed to update reconciliation with schedulingID '%s' "+
				"(maybe updated by parallel running process)", schedulingID)
		}

		return nil
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) GetReconciliations(filter Filter) ([]*model.ReconciliationEntity, error) {
	q, err := db.NewQuery(r.Conn, &model.ReconciliationEntity{}, r.Logger)
	if err != nil {
		return nil, err
	}

	//fire query
	selectQ := q.Select()
	if filter != nil {
		if err := filter.FilterByQuery(selectQ); err != nil {
			return nil, errors.Wrap(err, "failed to apply sql filter")
		}
	}
	// TODO: the ORM should maintain the correct SQL statement order and handle duplicated clauses.
	if !strings.Contains(selectQ.String(), "ORDER BY") {
		selectQ = selectQ.OrderBy(map[string]string{"Created": "DESC"})
	}
	recons, err := selectQ.GetMany()
	if err != nil {
		return nil, err
	}

	var result []*model.ReconciliationEntity
	for _, recon := range recons {
		result = append(result, recon.(*model.ReconciliationEntity))
	}
	return result, nil
}

func (r *PersistentReconciliationRepository) GetOperations(filter operation.Filter) ([]*model.OperationEntity, error) {
	q, err := db.NewQuery(r.Conn, &model.OperationEntity{}, r.Logger)
	if err != nil {
		return nil, err
	}

	//fire query
	selectQ := q.Select()
	if filter != nil {
		if err := filter.FilterByQuery(selectQ); err != nil {
			return nil, errors.Wrap(err, "failed to apply sql filter")
		}
	}
	ops, err := selectQ.GetMany()
	if err != nil {
		return nil, err
	}

	var result []*model.OperationEntity
	for _, op := range ops {
		result = append(result, op.(*model.OperationEntity))
	}
	return result, nil
}

func (r *PersistentReconciliationRepository) GetOperation(schedulingID, correlationID string) (*model.OperationEntity, error) {
	q, err := db.NewQuery(r.Conn, &model.OperationEntity{}, r.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"CorrelationID": correlationID,
		"SchedulingID":  schedulingID,
	}
	opEntity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, r.NewNotFoundError(err, opEntity, whereCond)
	}
	return opEntity.(*model.OperationEntity), nil
}

func (r *PersistentReconciliationRepository) GetProcessableOperations(maxParallelOpsPerRecon int) ([]*model.OperationEntity, error) {
	opEntities, err := r.GetReconcilingOperations()
	if err != nil {
		return nil, err
	}
	return findProcessableOperations(opEntities, maxParallelOpsPerRecon), nil
}

func (r *PersistentReconciliationRepository) GetReconcilingOperations() ([]*model.OperationEntity, error) {
	//retrieve all non-finished operations
	reconEntity := &model.ReconciliationEntity{}
	colHdr, err := db.NewColumnHandler(reconEntity, r.Conn, r.Logger)
	if err != nil {
		return nil, err
	}
	schedulingIDCol, err := colHdr.ColumnName("SchedulingID")
	if err != nil {
		return nil, err
	}
	FinishedCol, err := colHdr.ColumnName("Finished")
	if err != nil {
		return nil, err
	}
	q, err := db.NewQuery(r.Conn, &model.OperationEntity{}, r.Logger)
	if err != nil {
		return nil, err
	}
	ops, err := q.Select().
		WhereIn(
			"SchedulingID",
			//consider only operations which are part of a running reconciliations
			fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1", schedulingIDCol, reconEntity.Table(), FinishedCol), false).
		GetMany()
	if err != nil {
		return nil, err
	}

	var opEntities []*model.OperationEntity
	for _, op := range ops {
		opEntities = append(opEntities, op.(*model.OperationEntity))
	}
	return opEntities, nil
}

func (r *PersistentReconciliationRepository) UpdateOperationState(schedulingID, correlationID string, state model.OperationState, allowInState bool, reasons ...string) error {
	dbOps := func(tx *db.TxConnection) error {
		rTx, err := r.WithTx(tx)
		if err != nil {
			return err
		}
		op, err := rTx.GetOperation(schedulingID, correlationID)
		if err != nil {
			if repository.IsNotFoundError(err) {
				r.Logger.Warnf("ReconRepo could not find operation (schedulingID:%s/correlationID:%s)", schedulingID, correlationID)
			}
			return err
		}

		if op.State == state && !allowInState {
			return newAlreadyInStateError(op)
		}

		if op.State.IsFinal() {
			return fmt.Errorf("cannot update state of operation '%s' to new state '%s' "+
				"because operation is already in final state '%s'", op.Component, state, op.State)
		}

		//update operation-entity
		opStateOld := op.State //required in where-condition later on
		op.State = state
		reason, err := concatStateReasons(state, reasons)
		if err != nil {
			return err
		}
		op.Reason = reason
		op.Updated = time.Now().UTC()

		//prepare update query
		q, err := db.NewQuery(tx, op, r.Logger)
		if err != nil {
			return err
		}
		whereCond := map[string]interface{}{
			"CorrelationID": correlationID,
			"SchedulingID":  schedulingID,
			"State":         opStateOld, //ensure update will affect only operations which were not updated in between
		}

		update := q.Update().Where(whereCond)
		cnt, err := update.ExecCount()

		if err != nil {
			return err
		}

		if cnt == 0 {
			return fmt.Errorf("update of operation '%s' to state '%s' failed: no row was updated "+
				"(probably race-condition: operation does no longer match where-conditions)",
				op, state)
		}

		return nil
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) UpdateOperationRetryID(schedulingID, correlationID, retryID string) error {

	dbOps := func(tx *db.TxConnection) error {
		rTx, err := r.WithTx(tx)
		if err != nil {
			return err
		}
		op, err := rTx.GetOperation(schedulingID, correlationID)
		if err != nil {
			if repository.IsNotFoundError(err) {
				r.Logger.Warnf("ReconRepo could not find operation (schedulingID:%s/correlationID:%s)", schedulingID, correlationID)
			}
			return err
		}
		if retryID == op.RetryID {
			return nil
		}

		//update operation-entity
		op.RetryID = retryID
		op.Retries++
		op.Updated = time.Now().UTC()

		//prepare update query
		q, err := db.NewQuery(tx, op, r.Logger)
		if err != nil {
			return err
		}
		whereCond := map[string]interface{}{
			"CorrelationID": correlationID,
			"SchedulingID":  schedulingID,
		}
		cnt, err := q.Update().
			Where(whereCond).
			ExecCount()

		if cnt == 0 {
			return fmt.Errorf("update of operation '%s' retryID '%s' failed: no row was updated", op, retryID)
		}

		return err
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) UpdateOperationPickedUp(schedulingID, correlationID string) error {
	dbOps := func(tx *db.TxConnection) error {
		rTx, err := r.WithTx(tx)
		if err != nil {
			return err
		}
		op, err := rTx.GetOperation(schedulingID, correlationID)
		if err != nil {
			if repository.IsNotFoundError(err) {
				r.Logger.Warnf("ReconRepo could not find operation (schedulingID:%s/correlationID:%s)", schedulingID, correlationID)
			}
			return err
		}

		if !op.PickedUp.IsZero() {
			r.Logger.Debugf("Operation with schedulingID %s has initially been picked up at %s; PickedUp timestamp will not be updated", schedulingID, op.PickedUp)
			return nil // Not an error, since this happens when an operation was an orphan and gets picked up again
		}

		//update operation-entity
		op.PickedUp = time.Now().UTC()

		//prepare update query
		q, err := db.NewQuery(tx, op, r.Logger)
		if err != nil {
			return err
		}
		whereCond := map[string]interface{}{
			"CorrelationID": correlationID,
			"SchedulingID":  schedulingID,
		}
		cnt, err := q.Update().
			Where(whereCond).
			ExecCount()
		if cnt == 0 {
			return fmt.Errorf("update of operation '%s' pickedUp timestamp failed: no row was updated", op)
		}

		r.Logger.Debugf("Operation with schedulingID %s picked up at %s", schedulingID, op.PickedUp)

		return err
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) UpdateComponentOperationProcessingDuration(schedulingID, correlationID string, processingDuration int) error {
	dbOps := func(tx *db.TxConnection) error {
		rTx, err := r.WithTx(tx)
		if err != nil {
			return err
		}
		operations, err := rTx.GetOperations(&operation.FilterMixer{Filters: []operation.Filter{
			&operation.WithSchedulingID{
				SchedulingID: schedulingID,
			},
			&operation.WithCorrelationID{CorrelationID: correlationID},
		}})
		if err != nil {
			return err
		}
		if len(operations) != 1 {
			return fmt.Errorf("error finding operation with schedulingID %s and correlationID %s, found instead: %v", schedulingID, correlationID, operations)
		}
		operations[0].ProcessingDuration = int64(processingDuration)

		//prepare update query
		q, err := db.NewQuery(tx, operations[0], r.Logger)
		if err != nil {
			return err
		}
		whereCond := map[string]interface{}{
			"CorrelationID": correlationID,
			"SchedulingID":  schedulingID,
		}
		cnt, err := q.Update().
			Where(whereCond).
			ExecCount()
		if cnt == 0 {
			return fmt.Errorf("update of operation '%s' processingDuration failed: no row was updated", operations[0])
		}
		return err
	}
	return db.Transaction(r.Conn, dbOps, r.Logger)
}

func (r *PersistentReconciliationRepository) GetComponentOperationProcessingDuration(component string, state model.OperationState) (int64, error) {
	if state != model.OperationStateDone && state != model.OperationStateError {
		return 0, errors.Errorf("Unsupported Operation State %s for component %s", state, component)
	}

	operations, err := r.GetOperations(&operation.FilterMixer{Filters: []operation.Filter{
		&operation.WithComponentName{Component: component},
		&operation.WithStates{States: []model.OperationState{state}},
		&operation.LimitByLastUpdate{Count: 1},
	}})
	if err != nil {
		return 0, err
	}
	if len(operations) == 0 {
		r.Logger.Debugf("No operation for component %s found with desired state %s", component, state)
		return 0, nil
	}
	return operations[0].ProcessingDuration, nil
}

func (r *PersistentReconciliationRepository) GetMothershipOperationProcessingDuration(component string, state model.OperationState, startTime metricStartTime) (int64, error) {
	if state != model.OperationStateDone && state != model.OperationStateError {
		return 0, errors.Errorf("Unsupported Operation State: %s", state)
	}

	operations, err := r.GetOperations(&operation.FilterMixer{Filters: []operation.Filter{
		&operation.WithComponentName{Component: component},
		&operation.WithStates{States: []model.OperationState{state}},
		&operation.LimitByLastUpdate{Count: 1},
	}})
	if err != nil {
		return 0, err
	}
	if len(operations) == 0 {
		r.Logger.Debugf("No operation for component %s found with desired state %s", component, state)
		return 0, nil
	}
	var duration time.Duration
	op := operations[0]

	switch startTime {
	case Created:
		duration = op.Updated.Sub(op.Created)
	case PickedUp:
		duration = op.Updated.Sub(op.PickedUp)
	}
	return duration.Milliseconds(), nil
}

func (r *PersistentReconciliationRepository) GetAllComponents() ([]string, error) {
	opEntity := &model.OperationEntity{}
	colHdr, err := db.NewColumnHandler(opEntity, r.Conn, r.Logger)
	if err != nil {
		return nil, err
	}
	componentCol, err := colHdr.ColumnName("Component")
	if err != nil {
		return nil, err
	}

	q, err := db.NewQuery(r.Conn, opEntity, r.Logger)
	if err != nil {
		return nil, err
	}

	dataRows, err := q.Conn.Query(fmt.Sprintf("SELECT %s FROM %s GROUP BY %s", componentCol, opEntity.Table(), componentCol))
	if err != nil {
		return nil, err
	}
	var components []string

	for dataRows.Next() {
		var component string
		err := dataRows.Scan(&component)
		if err == nil {
			components = append(components, component)
		}
	}

	return components, nil
}
