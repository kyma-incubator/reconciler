package cluster

import (
	"bytes"
	"database/sql"
	"fmt"
	"gorm.io/gorm"
	"time"

	"github.com/pkg/errors"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
)

type Inventory interface {
	CreateOrUpdate(contractVersion int64, cluster *keb.Cluster) (*State, error)
	UpdateStatus(State *State, status model.Status) (*State, error)
	MarkForDeletion(runtimeID string) (*State, error)
	Delete(runtimeID string) error
	Get(runtimeID string, configVersion int64) (*State, error)
	GetLatest(runtimeID string) (*State, error)
	GetAll() ([]*State, error)
	StatusChanges(runtimeID string, offset time.Duration) ([]*StatusChange, error)
	ClustersToReconcile(reconcileInterval time.Duration) ([]*State, error)
	ClustersNotReady() ([]*State, error)
	CountRetries(runtimeID string, configVersion int64, maxRetries int, errorStatus ...model.Status) (int, error)
	WithTx(tx *db.TxConnection) (Inventory, error)
}

type DefaultInventory struct {
	*repository.Repository
	metricsCollector
}

type metricsCollector interface {
	OnClusterStateUpdate(state *State) error
}

type clusterStatusIdent struct {
	clusterVersion int64
	configVersion  int64
}

func NewInventory(conn db.Connection, debug bool, collector metricsCollector) (Inventory, error) {
	repo, err := repository.NewRepository(conn, debug)
	if err != nil {
		return nil, err
	}
	return &DefaultInventory{repo, collector}, nil
}

func (i *DefaultInventory) WithTx(tx *db.TxConnection) (Inventory, error) {
	return NewInventory(tx, i.Debug, i.metricsCollector)
}

// Used as tables for GORM Query
type inventoryClusters struct{}
type inventoryClusterConfigs struct{}
type inventoryClusterConfigStatus struct{}

func (i *DefaultInventory) CountRetries(runtimeID string, configVersion int64, maxRetries int, errorStatus ...model.Status) (int, error) {
	if len(errorStatus) == 0 {
		return 0, errors.New("errorStatus slice is empty")
	}

	var maxStatusHistoryLength = maxRetries * 5 //cluster can have three interims state between errors, thus 5 is more than enough
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterStatusEntity{}, i.Logger)

	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("failed to initialize query for runtime %s", runtimeID))
	}
	clusterStatusesSQL := q.Query().Select("*").
		Where("runtime_id = @runtime AND config_version = @configversion", sql.Named("runtime", runtimeID), sql.Named("configversion", configVersion)).
		Order("id desc").
		Limit(maxStatusHistoryLength).
		Find(inventoryClusterConfigStatus{})
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("failed to count error for runtime %s", runtimeID))
	}
	dataRows, err := i.Conn.QueryGorm(clusterStatusesSQL)
	if err != nil {
		return 0, err
	}

	errCnt := 0
	for dataRows.Next() {
		var clusterStatusEntity model.ClusterStatusEntity
		if err := dataRows.Scan(&clusterStatusEntity.ID,
			&clusterStatusEntity.RuntimeID,
			&clusterStatusEntity.ClusterVersion,
			&clusterStatusEntity.ConfigVersion,
			&clusterStatusEntity.Status,
			&clusterStatusEntity.Created,
			&clusterStatusEntity.Deleted); err != nil {
			return 0, errors.Wrap(err, "failed to bind cluster-status-idents")
		}
		if clusterStatusEntity.Status.IsFinal() {
			if statusInSlice(clusterStatusEntity.Status, errorStatus) {
				errCnt++
			} else if clusterStatusEntity.Status.IsFinalStable() {
				break
			}
		}
	}
	return errCnt, nil
}

func statusInSlice(status model.Status, statusList []model.Status) bool {
	for _, s := range statusList {
		if s == status {
			return true
		}
	}
	return false
}

func (i *DefaultInventory) CreateOrUpdate(contractVersion int64, cluster *keb.Cluster) (*State, error) {
	if len(cluster.KymaConfig.Components) == 0 {
		return nil, fmt.Errorf("error creating cluster with RuntimeID: %s, component list is empty", cluster.RuntimeID)
	}
	dbOps := func(tx *db.TxConnection) (interface{}, error) {
		var iTx *DefaultInventory
		tmpiTx, err := i.WithTx(tx)
		if err != nil {
			return nil, err
		}
		iTx = tmpiTx.(*DefaultInventory)
		clusterEntity, err := iTx.createCluster(contractVersion, cluster)
		if err != nil {
			return nil, err
		}
		clusterConfigurationEntity, err := iTx.createConfiguration(contractVersion, cluster, clusterEntity)
		if err != nil {
			return nil, err
		}
		clusterStatusEntity, err := iTx.createStatus(clusterConfigurationEntity, model.ClusterStatusReconcilePending)
		if err != nil {
			return nil, err
		}
		return &State{
			Cluster:       clusterEntity,
			Configuration: clusterConfigurationEntity,
			Status:        clusterStatusEntity,
		}, nil

	}

	state, err := db.TransactionResult(i.Conn, dbOps, i.Logger)
	if err != nil {
		i.Logger.Errorf("Inventory failed to create/update cluster with runtimeID '%s': %s", cluster.RuntimeID, err)
		return nil, err
	}

	stateEntity := state.(*State)
	err = i.metricsCollector.OnClusterStateUpdate(stateEntity)
	if err != nil {
		return nil, err
	}

	i.Logger.Infof("Inventory created/updated cluster with runtimeID '%s' "+
		"(clusterVersion:%d/configVersion:%d/status:%s)",
		stateEntity.Cluster.RuntimeID,
		stateEntity.Cluster.Version, stateEntity.Configuration.Version, stateEntity.Status.Status)

	return stateEntity, nil
}

func (i *DefaultInventory) createCluster(contractVersion int64, cluster *keb.Cluster) (*model.ClusterEntity, error) {
	newClusterEntity := &model.ClusterEntity{
		RuntimeID:  cluster.RuntimeID,
		Runtime:    &cluster.RuntimeInput,
		Metadata:   &cluster.Metadata,
		Kubeconfig: cluster.Kubeconfig,
		Contract:   contractVersion,
	}

	//check if a new version is required
	oldClusterEntity, err := i.latestCluster(cluster.RuntimeID)
	if err == nil {
		if oldClusterEntity.Equal(newClusterEntity) { //reuse existing cluster entity
			i.Logger.Debugf("No differences found for cluster '%s': not creating new database entity", cluster.RuntimeID)
			return oldClusterEntity, nil
		}
	} else if !repository.IsNotFoundError(err) {
		//unexpected error
		return nil, err
	}

	//create new version
	q, err := db.NewQueryGorm(i.Conn, newClusterEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	newDbEntity, err := q.Insert(inventoryClusters{})
	if err != nil {
		return nil, err
	}
	return newDbEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) createConfiguration(contractVersion int64, cluster *keb.Cluster, clusterEntity *model.ClusterEntity) (*model.ClusterConfigurationEntity, error) {
	newConfigEntity := &model.ClusterConfigurationEntity{
		RuntimeID:      clusterEntity.RuntimeID,
		ClusterVersion: clusterEntity.Version,
		KymaVersion:    cluster.KymaConfig.Version,
		KymaProfile:    cluster.KymaConfig.Profile,
		Components: func() []*keb.Component {
			var result []*keb.Component
			for idx := range cluster.KymaConfig.Components {
				result = append(result, &cluster.KymaConfig.Components[idx])
			}
			return result
		}(),
		Administrators: cluster.KymaConfig.Administrators,
		Contract:       contractVersion,
	}

	//check if a new version is required
	oldConfigEntity, err := i.latestConfig(clusterEntity.Version)
	if err == nil {
		if oldConfigEntity.Equal(newConfigEntity) { //reuse existing config entity
			i.Logger.Debugf("No differences found for configuration of cluster '%s': not creating new database entity", cluster.RuntimeID)
			return oldConfigEntity, nil
		}
	} else if !repository.IsNotFoundError(err) {
		//unexpected error
		return nil, err
	}

	//create new version
	q, err := db.NewQueryGorm(i.Conn, newConfigEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	newDbEntity, err := q.Insert(inventoryClusterConfigs{})
	if err != nil {
		return nil, err
	}
	return newDbEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) createStatus(configEntity *model.ClusterConfigurationEntity, status model.Status) (*model.ClusterStatusEntity, error) {
	newStatusEntity := &model.ClusterStatusEntity{
		RuntimeID:      configEntity.RuntimeID,
		ClusterVersion: configEntity.ClusterVersion,
		ConfigVersion:  configEntity.Version,
		Status:         status,
	}

	//check if a new version is required
	oldStatusEntity, err := i.latestStatus(configEntity.Version)
	if err == nil {
		if oldStatusEntity.Equal(newStatusEntity) { //reuse existing status entity
			i.Logger.Debugf("No differences found for status of cluster '%s': not creating new database entity", configEntity.RuntimeID)
			return oldStatusEntity, nil
		}
	} else if !repository.IsNotFoundError(err) {
		//unexpected error
		return nil, err
	}

	//create new status
	q, err := db.NewQueryGorm(i.Conn, newStatusEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	newDbEntity, err := q.Insert(inventoryClusterConfigStatus{})
	if err != nil {
		return nil, err
	}

	return newDbEntity.(*model.ClusterStatusEntity), nil
}

func (i *DefaultInventory) UpdateStatus(state *State, status model.Status) (*State, error) {
	newStatus, err := i.createStatus(state.Configuration, status)
	if err != nil {
		return state, err
	}
	state.Status = newStatus
	err = i.metricsCollector.OnClusterStateUpdate(state)
	if err != nil {
		return state, err
	}
	return state, nil
}

func (i *DefaultInventory) MarkForDeletion(runtimeID string) (*State, error) {
	clusterState, err := i.GetLatest(runtimeID)
	if err != nil {
		return nil, err
	}
	return i.UpdateStatus(clusterState, model.ClusterStatusDeletePending)
}

func (i *DefaultInventory) Delete(runtimeID string) error {
	dbOps := func(tx *db.TxConnection) error {
		newClusterName := fmt.Sprintf("deleted_%d_%s", time.Now().Unix(), runtimeID) //TODO: Fuur gorm anpassen aus convinience
		updateSQLTpl := "UPDATE %s SET %s=$1, %s=$2 WHERE %s=$3 OR %s=$4"            //OR condition required for Postgres: new cluster-name is automatically cascaded to config-status table

		//update name of all cluster entities
		clusterEntity := &model.ClusterEntity{}
		clusterColHandler, err := db.NewColumnHandler(clusterEntity, i.Conn, i.Logger)
		if err != nil {
			return err
		}
		clusterColName, err := clusterColHandler.ColumnName("RuntimeID")
		if err != nil {
			return err
		}
		clusterDelColName, err := clusterColHandler.ColumnName("Deleted")
		if err != nil {
			return err
		}
		clusterUpdateSQL := fmt.Sprintf(updateSQLTpl, clusterEntity.Table(), clusterColName, clusterDelColName, clusterColName, clusterColName)
		if _, err := tx.Exec(clusterUpdateSQL, newClusterName, "TRUE", runtimeID, newClusterName); err != nil {
			return err
		}

		//update cluster-name of all referenced cluster-config entities
		configEntity := &model.ClusterConfigurationEntity{}
		configColHandler, err := db.NewColumnHandler(configEntity, i.Conn, i.Logger)
		if err != nil {
			return err
		}
		configClusterColName, err := configColHandler.ColumnName("RuntimeID")
		if err != nil {
			return err
		}
		configDelColName, err := configColHandler.ColumnName("Deleted")
		if err != nil {
			return err
		}
		configUpdateSQL := fmt.Sprintf(updateSQLTpl, configEntity.Table(), configClusterColName, configDelColName, configClusterColName, configClusterColName)
		if _, err := tx.Exec(configUpdateSQL, newClusterName, "TRUE", runtimeID, newClusterName); err != nil {
			return err
		}

		//update cluster-name of all referenced cluster-status entities
		statusEntity := &model.ClusterStatusEntity{}
		statusColHandler, err := db.NewColumnHandler(statusEntity, i.Conn, i.Logger)
		if err != nil {
			return err
		}
		statusClusterColName, err := statusColHandler.ColumnName("RuntimeID")
		if err != nil {
			return err
		}
		statusDelColName, err := statusColHandler.ColumnName("Deleted")
		if err != nil {
			return err
		}
		statusUpdateSQL := fmt.Sprintf(updateSQLTpl, statusEntity.Table(), statusClusterColName, statusDelColName, statusClusterColName, statusClusterColName)
		if _, err := tx.Exec(statusUpdateSQL, newClusterName, "TRUE", runtimeID, newClusterName); err != nil {
			return err
		}

		//done
		return nil
	}
	err := db.Transaction(i.Conn, dbOps, i.Logger)
	if err == nil {
		i.Logger.Infof("Inventory deleted cluster with runtimeID '%s'", runtimeID)
	} else {
		i.Logger.Errorf("Inventory failed to delete cluster with runtimeID '%s': %s", runtimeID, err)
	}
	return err
}

func (i *DefaultInventory) Get(runtimeID string, configVersion int64) (*State, error) {
	configEntity, err := i.config(runtimeID, configVersion)
	if err != nil {
		return nil, err
	}
	statusEntity, err := i.latestStatus(configVersion)
	if err != nil {
		return nil, err
	}
	clusterEntity, err := i.cluster(configEntity.ClusterVersion)
	if err != nil {
		return nil, err
	}
	return &State{
		Cluster:       clusterEntity,
		Configuration: configEntity,
		Status:        statusEntity,
	}, nil
}

func (i *DefaultInventory) GetLatest(runtimeID string) (*State, error) {
	clusterEntity, err := i.latestCluster(runtimeID)
	if err != nil {
		return nil, err
	}
	configEntity, err := i.latestConfig(clusterEntity.Version)
	if err != nil {
		return nil, err
	}
	statusEntity, err := i.latestStatus(configEntity.Version)
	if err != nil {
		return nil, err
	}

	return &State{
		Cluster:       clusterEntity,
		Configuration: configEntity,
		Status:        statusEntity,
	}, nil
}

func (i *DefaultInventory) GetAll() ([]*State, error) {
	return i.filterClusters()
}

func (i *DefaultInventory) latestStatus(configVersion int64) (*model.ClusterStatusEntity, error) {
	whereCond := map[string]interface{}{
		"config_version": configVersion,
	}
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterStatusEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	latestStatus, err := q.GetOne(whereCond, "id desc", inventoryClusterConfigStatus{})
	if err != nil {
		return nil, i.MapError(err, latestStatus, whereCond)
	}
	return q.DbEntity.(*model.ClusterStatusEntity), nil
}

func (i *DefaultInventory) config(runtimeID string, configVersion int64) (*model.ClusterConfigurationEntity, error) {
	whereCond := map[string]interface{}{
		"version":    configVersion,
		"runtime_id": runtimeID,
	}
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterConfigurationEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	configEntity, err := q.GetOne(whereCond, "version desc", inventoryClusterConfigs{})
	if err != nil {
		return nil, i.MapError(err, configEntity, whereCond)
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) latestConfig(clusterVersion int64) (*model.ClusterConfigurationEntity, error) {
	whereCond := map[string]interface{}{
		"cluster_version": clusterVersion,
	}
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterConfigurationEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	configEntity, err := q.GetOne(whereCond, "version desc", inventoryClusterConfigs{})
	if err != nil {
		return nil, i.MapError(err, configEntity, whereCond)
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) cluster(clusterVersion int64) (*model.ClusterEntity, error) {
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"version": clusterVersion,
		"deleted": false,
	}
	clusterEntity, err := q.GetOne(whereCond, "version desc", inventoryClusters{})
	if err != nil {
		return nil, i.MapError(err, clusterEntity, whereCond)
	}
	return clusterEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) latestCluster(runtimeID string) (*model.ClusterEntity, error) {
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"runtime_id": runtimeID,
		"deleted":    false,
	}
	clusterEntity, err := q.GetOne(whereCond, "version desc", inventoryClusters{})
	if err != nil {
		return nil, i.MapError(err, q.DbEntity, whereCond)
	}

	return clusterEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) ClustersToReconcile(reconcileInterval time.Duration) ([]*State, error) {
	var filters []statusSQLFilter
	if reconcileInterval > 0 {
		filters = append(filters, &reconcileIntervalFilter{
			reconcileInterval: reconcileInterval,
		})
	}
	filters = append(filters, &statusFilter{
		allowedStatuses: []model.Status{model.ClusterStatusReconcilePending, model.ClusterStatusDeletePending},
	})
	return i.filterClusters(filters...)
}

func (i *DefaultInventory) ClustersNotReady() ([]*State, error) {
	statusFilter := &statusFilter{
		allowedStatuses: []model.Status{
			model.ClusterStatusReconcileError, model.ClusterStatusReconcileErrorRetryable,
			model.ClusterStatusDeleting, model.ClusterStatusDeleteError, model.ClusterStatusDeleteErrorRetryable},
	}
	return i.filterClusters(statusFilter)
}

func (i *DefaultInventory) filterClusters(filters ...statusSQLFilter) ([]*State, error) {
	//get DDL for sub-query
	clusterStatusEntity := &model.ClusterStatusEntity{}

	statusColHandler, err := db.NewColumnHandler(clusterStatusEntity, i.Conn, i.Logger)
	if err != nil {
		return nil, err
	}
	idColName, err := statusColHandler.ColumnName("ID")
	if err != nil {
		return nil, err
	}
	runtimeIDColName, err := statusColHandler.ColumnName("RuntimeID")
	if err != nil {
		return nil, err
	}
	clusterVersionColName, err := statusColHandler.ColumnName("ClusterVersion")
	if err != nil {
		return nil, err
	}
	configVersionColName, err := statusColHandler.ColumnName("ConfigVersion")
	if err != nil {
		return nil, err
	}
	deletedColName, err := statusColHandler.ColumnName("Deleted")
	if err != nil {
		return nil, err
	}

	q, err := db.NewQueryGorm(i.Conn, clusterStatusEntity, i.Logger)
	if err != nil {
		return nil, err
	}

	columnMap := map[string]string{ //just for convenience to avoid longer parameter lists
		"ID":             idColName,
		"RuntimeID":      runtimeIDColName,
		"ClusterVersion": clusterVersionColName,
		"ConfigVersion":  configVersionColName,
		"Deleted":        deletedColName,
	}
	statusIdsSQL, err := i.buildLatestStatusIdsSQL(columnMap, clusterStatusEntity)
	if err != nil {
		return nil, err
	}
	if db.GetString(statusIdsSQL) == "" { //no status entities found to reconcile
		return nil, nil
	}

	statusFilterSQL, err := i.buildStatusFilterSQL(filters, statusColHandler)
	if err != nil {
		return nil, err
	}

	filterSQL := q.Query().Select("*").
		Where("id IN (?)", statusIdsSQL). //query latest cluster states (= max(configVersion) within max(clusterVersion))
		Where(statusFilterSQL).           //filter these states also by provided criteria (by statuses, reconcile-interval etc.)
		Where(map[string]interface{}{"deleted": false}).
		Find(inventoryClusterConfigStatus{})

	dataRows, err := i.Conn.QueryGorm(filterSQL)
	if err != nil {
		return nil, err
	}
	var clusterStatuses []model.ClusterStatusEntity
	for dataRows.Next() {
		var clusterStatusEntity model.ClusterStatusEntity
		if err := dataRows.Scan(&clusterStatusEntity.ID,
			&clusterStatusEntity.RuntimeID,
			&clusterStatusEntity.ClusterVersion,
			&clusterStatusEntity.ConfigVersion,
			&clusterStatusEntity.Status,
			&clusterStatusEntity.Created,
			&clusterStatusEntity.Deleted); err != nil {
			return nil, errors.Wrap(err, "failed to bind cluster-status-idents")
		}
		clusterStatuses = append(clusterStatuses, clusterStatusEntity)
	}

	//retrieve clusters which require a reconciliation
	var result []*State
	for _, clusterStatus := range clusterStatuses {
		state, err := i.Get(clusterStatus.RuntimeID, clusterStatus.ConfigVersion)
		if err != nil {
			return nil, err
		}
		result = append(result, state)
	}

	return result, nil
}

func (i *DefaultInventory) buildLatestStatusIdsSQL(columnMap map[string]string, clusterStatusEntity *model.ClusterStatusEntity) (*gorm.DB, error) {
	//SQL to retrieve the latest statuses => max(config_version) within max(cluster_version):
	/*
		select cluster_version, max(config_version) from inventory_cluster_config_statuses where cluster_version in (
			select max(cluster_version) from inventory_cluster_config_statuses where deleted = false group by runtime_id
		) group by cluster_version
	*/
	dataRows, err := i.Conn.Query(
		fmt.Sprintf( // TODO: Rewrite with grom to stay consistend
			"SELECT %s, MAX(%s) FROM %s WHERE %s IN (SELECT MAX(%s) FROM %s WHERE %s=$1 GROUP BY %s) GROUP BY %s ",
			columnMap["ClusterVersion"], columnMap["ConfigVersion"], clusterStatusEntity.Table(), columnMap["ClusterVersion"],
			columnMap["ClusterVersion"], clusterStatusEntity.Table(), columnMap["Deleted"], columnMap["RuntimeID"],
			columnMap["ClusterVersion"]),
		false)

	if err != nil {
		return &gorm.DB{}, errors.Wrap(err, "failed to retrieve cluster-status-idents")
	}

	//SQL to retrieve entity-IDs for previously retrieved latest statuses:
	/*
		select max(id) from inventory_cluster_config_statuses where
			(cluster_version=x and config_version=y) or (cluster_version=a and config_version=v) or ...
		 group by cluster_version
	*/
	q, err := db.NewQueryGorm(i.Conn, &model.ClusterStatusEntity{}, i.Logger)
	if err != nil {
		return &gorm.DB{}, err
	}
	subquery := q.Query().Select("MAX(id)")
	whereClause := false
	for dataRows.Next() {
		var row clusterStatusIdent
		if err := dataRows.Scan(&row.clusterVersion, &row.configVersion); err != nil {
			return subquery, errors.Wrap(err, "failed to bind cluster-status-idents")
		}
		if whereClause { // length of gormDB Statement Vars cannot be used, because Vars slice is empty until Find() gets called
			subquery = subquery.Or(fmt.Sprintf("(%s=@clusterversion AND %s=@configversion)", columnMap["ClusterVersion"], columnMap["ConfigVersion"]),
				sql.Named("clusterversion", row.clusterVersion),
				sql.Named("configversion", row.configVersion))
		} else {
			whereClause = true
			subquery = subquery.Where("cluster_version=@clusterversion AND config_version=@configversion",
				sql.Named("clusterversion", row.clusterVersion),
				sql.Named("configversion", row.configVersion))
		}
	}
	subquery = subquery.Group(columnMap["ClusterVersion"]).Find(inventoryClusterConfigStatus{})

	return subquery, nil
}

func (i *DefaultInventory) buildStatusFilterSQL(filters []statusSQLFilter, statusColHandler *db.ColumnHandler) (string, error) {
	var sqlFilterStmt bytes.Buffer
	if len(filters) == 0 {
		sqlFilterStmt.WriteString("1=1") //if no filters are provided, use 1=1 as placeholder to ensure valid SQL query
	}
	for _, filter := range filters {
		sqlCond, err := filter.Filter(i.Conn.Type(), statusColHandler)
		if err != nil {
			return "", err
		}
		if sqlFilterStmt.Len() > 0 {
			sqlFilterStmt.WriteString(" OR ")
		}
		sqlFilterStmt.WriteRune('(')
		sqlFilterStmt.WriteString(sqlCond)
		sqlFilterStmt.WriteRune(')')
	}
	return sqlFilterStmt.String(), nil
}

func (i *DefaultInventory) StatusChanges(runtimeID string, offset time.Duration) ([]*StatusChange, error) {
	clusterStatusEntity := &model.ClusterStatusEntity{}

	//build sub-query
	statusColHandler, err := db.NewColumnHandler(clusterStatusEntity, i.Conn, i.Logger)
	if err != nil {
		return nil, err
	}

	filter := createdIntervalFilter{
		interval:  offset,
		runtimeID: runtimeID,
	}
	sqlCond, err := filter.Filter(i.Conn.Type(), statusColHandler)
	if err != nil {
		return nil, err
	}

	//query status entities
	q, err := db.NewQueryGorm(i.Conn, clusterStatusEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	subquery := q.Query().Select("id").
		Where(sqlCond).
		Find(inventoryClusterConfigStatus{})
	statusEnitySQL := q.Query().Select("*").
		Where("id IN (?)", subquery).
		Order("id desc").Find(inventoryClusterConfigStatus{})

	dataRows, err := i.Conn.QueryGorm(statusEnitySQL)
	if err != nil {
		return nil, err
	}
	var clusterStatuses []model.ClusterStatusEntity
	for dataRows.Next() {
		var clusterStatusEntity model.ClusterStatusEntity
		if err := dataRows.Scan(&clusterStatusEntity.ID,
			&clusterStatusEntity.RuntimeID,
			&clusterStatusEntity.ClusterVersion,
			&clusterStatusEntity.ConfigVersion,
			&clusterStatusEntity.Status,
			&clusterStatusEntity.Created,
			&clusterStatusEntity.Deleted); err != nil {
			return nil, errors.Wrap(err, "failed to bind cluster-status-idents")
		}
		clusterStatuses = append(clusterStatuses, clusterStatusEntity)
	}
	if len(clusterStatuses) == 0 {
		//invalid state: there cannot be a cluster without any state
		return nil, i.NewNotFoundError(
			fmt.Errorf("no status found for cluster '%s'", runtimeID),
			clusterStatusEntity,
			map[string]interface{}{
				"RuntimeID": runtimeID,
			})
	}

	//build list of status changes
	var statusChanges []*StatusChange
	var createdPrevStatus time.Time
	for _, clusterStatus := range clusterStatuses {
		var duration time.Duration
		if createdPrevStatus.IsZero() {
			duration = time.Since(clusterStatus.Created)
		} else {
			duration = createdPrevStatus.Sub(clusterStatus.Created)
		}
		newClusterStatus := clusterStatus
		statusChanges = append(statusChanges, &StatusChange{
			Status:   &newClusterStatus,
			Duration: duration,
		})

		createdPrevStatus = clusterStatus.Created
	}

	return statusChanges, nil
}
