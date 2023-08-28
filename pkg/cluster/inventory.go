package cluster

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	RemoveStatusesWithoutReconciliations(timeout time.Duration, statusCleanupBatchSize int) (int, error)
	RemoveDeletedClustersOlderThan(deadline time.Time) (int, error)
}

type DefaultInventory struct {
	*repository.Repository
	metricsCollector
	clientSet *kubernetes.Clientset
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

	var clientSet *kubernetes.Clientset
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		repo.Logger.Warn("Cluster inventory failed to create local Kubernetes config: %s", err)
	} else {
		clientSet, err = kubernetes.NewForConfig(config)
		if err == nil {
			repo.Logger.Info("Cluster inventory successfully created a local cluster client")
		} else {
			repo.Logger.Warn("Cluster inventory failed to create local Kubernetes clientSet: %s", err)
		}
	}

	return &DefaultInventory{repo, collector, clientSet}, nil
}

func (i *DefaultInventory) WithTx(tx *db.TxConnection) (Inventory, error) {
	return NewInventory(tx, i.Debug, i.metricsCollector)
}

func (i *DefaultInventory) CountRetries(runtimeID string, configVersion int64, maxRetries int, errorStatus ...model.Status) (int, error) {
	if len(errorStatus) == 0 {
		return 0, errors.New("errorStatus slice is empty")
	}

	var maxStatusHistoryLength = maxRetries * 5 //cluster can have three interims state between errors, thus 5 is more than enough
	q, err := db.NewQuery(i.Conn, &model.ClusterStatusEntity{}, i.Logger)
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("failed to initialize query for runtime %s", runtimeID))
	}
	clusterStatuses, err := q.Select().Where(map[string]interface{}{"RuntimeID": runtimeID, "ConfigVersion": configVersion}).OrderBy(map[string]string{"ID": "desc"}).Limit(maxStatusHistoryLength).GetMany()
	if err != nil {
		return 0, errors.Wrap(err, fmt.Sprintf("failed to count error for runtime %s", runtimeID))
	}

	errCnt := 0
	for _, clusterStatus := range clusterStatuses {
		clStatusEntity := clusterStatus.(*model.ClusterStatusEntity)
		if clStatusEntity.Status.IsFinal() {
			if statusInSlice(clStatusEntity.Status, errorStatus) {
				errCnt++
			} else if clStatusEntity.Status.IsFinalStable() {
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
	result, err := i.getOrCreateCluster(contractVersion, cluster)
	if err != nil {
		return result, err
	}

	//TODO: clarify whether this is the right lookup!
	if i.clientSet != nil {
		secret, err := i.clientSet.CoreV1().Secrets("kcp-system").Get(context.TODO(), result.RuntimeID, v1.GetOptions{})
		if err != nil {
			if k8serr.IsNotFound(err) {
				i.Logger.Debugf("Cluster inventory cannot find a kubeconfig-secret for cluster with runtimeID %s", result.RuntimeID)
			} else {
				i.Logger.Errorf("Cluster inventory failed to lookup kubeconfig-secret for cluster with runtimeID %s: %s", result.RuntimeID, err)
			}
			return result, err
		}

		if kubeconfig, found := secret.StringData["kubeconfig"]; !found {
			i.Logger.Errorf("Kubeconfig-secret for runtime '%s' does not include the data-key 'kubeconfig'", result.RuntimeID)
		} else {
			i.Logger.Debug("Overwriting kubeconfig of cluster (runtimeID: %s) with value from kubeconfig-secret")
			result.Kubeconfig = kubeconfig
		}
	}

	return result, err
}

func (i *DefaultInventory) getOrCreateCluster(contractVersion int64, cluster *keb.Cluster) (*model.ClusterEntity, error) {
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
	q, err := db.NewQuery(i.Conn, newClusterEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}

	return newClusterEntity, nil
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
	q, err := db.NewQuery(i.Conn, newConfigEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}

	return newConfigEntity, nil
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
	q, err := db.NewQuery(i.Conn, newStatusEntity, i.Logger)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}

	return newStatusEntity, nil
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
		newClusterName := fmt.Sprintf("deleted_%d_%s", time.Now().Unix(), runtimeID)
		updateSQLTpl := "UPDATE %s SET %s=$1, %s=$2 WHERE %s=$3 OR %s=$4" //OR condition required for Postgres: new cluster-name is automatically cascaded to config-status table

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
	q, err := db.NewQuery(i.Conn, &model.ClusterStatusEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"ConfigVersion": configVersion,
	}
	statusEntity, err := q.Select().
		Where(whereCond).
		OrderBy(map[string]string{"ID": "desc"}).
		GetOne()
	if err != nil {
		return nil, i.MapError(err, statusEntity, whereCond)
	}
	return statusEntity.(*model.ClusterStatusEntity), nil
}

func (i *DefaultInventory) config(runtimeID string, configVersion int64) (*model.ClusterConfigurationEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterConfigurationEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"Version":   configVersion,
		"RuntimeID": runtimeID,
	}
	configEntity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, i.MapError(err, configEntity, whereCond)
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) latestConfig(clusterVersion int64) (*model.ClusterConfigurationEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterConfigurationEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"ClusterVersion": clusterVersion,
	}
	configEntity, err := q.Select().
		Where(whereCond).
		OrderBy(map[string]string{"Version": "desc"}).
		GetOne()
	if err != nil {
		return nil, i.MapError(err, configEntity, whereCond)
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) cluster(clusterVersion int64) (*model.ClusterEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"Version": clusterVersion,
		"Deleted": false,
	}
	clusterEntity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, i.MapError(err, clusterEntity, whereCond)
	}
	return clusterEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) latestCluster(runtimeID string) (*model.ClusterEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"RuntimeID": runtimeID,
		"Deleted":   false,
	}
	clusterEntity, err := q.Select().
		Where(whereCond).
		OrderBy(map[string]string{
			"Version": "desc",
		}).
		GetOne()
	if err != nil {
		return nil, i.MapError(err, clusterEntity, whereCond)
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

	q, err := db.NewQuery(i.Conn, clusterStatusEntity, i.Logger)
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
	statusIdsSQL, statusIdsArgs, err := i.buildLatestStatusIdsSQL(columnMap, clusterStatusEntity)
	if err != nil {
		return nil, err
	}
	if statusIdsSQL == "" { //no status entities found to reconcile
		return nil, nil
	}

	statusFilterSQL, err := i.buildStatusFilterSQL(filters, statusColHandler)
	if err != nil {
		return nil, err
	}

	clusterStatuses, err := q.Select().
		WhereIn("ID", statusIdsSQL, statusIdsArgs...). //query latest cluster states (= max(configVersion) within max(clusterVersion))
		WhereRaw(statusFilterSQL).                     //filter these states also by provided criteria (by statuses, reconcile-interval etc.)
		Where(map[string]interface{}{"Deleted": false}).
		GetMany()
	if err != nil {
		return nil, err
	}

	//retrieve clusters which require a reconciliation
	var result []*State
	for _, clusterStatus := range clusterStatuses {
		clStateEntity := clusterStatus.(*model.ClusterStatusEntity)
		state, err := i.Get(clStateEntity.RuntimeID, clStateEntity.ConfigVersion)
		if err != nil {
			return nil, err
		}
		result = append(result, state)
	}

	return result, nil
}

func (i *DefaultInventory) buildLatestStatusIdsSQL(columnMap map[string]string, clusterStatusEntity *model.ClusterStatusEntity) (string, []interface{}, error) {
	var args []interface{}

	//SQL to retrieve the latest statuses => max(config_version) within max(cluster_version):
	/*
		select cluster_version, max(config_version) from inventory_cluster_config_statuses where cluster_version in (
			select max(cluster_version) from inventory_cluster_config_statuses group by runtime_id
		) group by cluster_version
	*/
	dataRows, err := i.Conn.Query(
		fmt.Sprintf(
			"SELECT %s, MAX(%s) FROM %s WHERE %s IN (SELECT MAX(%s) FROM %s WHERE %s=$1 GROUP BY %s) GROUP BY %s ",
			columnMap["ClusterVersion"], columnMap["ConfigVersion"], clusterStatusEntity.Table(), columnMap["ClusterVersion"],
			columnMap["ClusterVersion"], clusterStatusEntity.Table(), columnMap["Deleted"], columnMap["RuntimeID"],
			columnMap["ClusterVersion"]),
		false)

	if err != nil {
		return "", args, errors.Wrap(err, "failed to retrieve cluster-status-idents")
	}

	//SQL to retrieve entity-IDs for previously retrieved latest statuses:
	/*
		select max(id) from inventory_cluster_config_statuses where
			(cluster_version=x and config_version=y) or (cluster_version=a and config_version=v) or ...
		 group by cluster_version
	*/
	var subQuery bytes.Buffer
	subQuery.WriteString(fmt.Sprintf("SELECT MAX(%s) FROM %s WHERE ", columnMap["ID"], clusterStatusEntity.Table()))
	for dataRows.Next() {
		if len(args) > 0 {
			subQuery.WriteString(" OR ")
		}
		subQuery.WriteRune('(')
		var row clusterStatusIdent
		if err := dataRows.Scan(&row.clusterVersion, &row.configVersion); err != nil {
			return "", args, errors.Wrap(err, "failed to bind cluster-status-idents")
		}
		subQuery.WriteString(fmt.Sprintf("%s=$%d AND %s=$%d",
			columnMap["ClusterVersion"], len(args)+1,
			columnMap["ConfigVersion"], len(args)+2))
		args = append(args, row.clusterVersion, row.configVersion)
		subQuery.WriteRune(')')
	}
	subQuery.WriteString(fmt.Sprintf(" GROUP BY %s", columnMap["ClusterVersion"]))

	if len(args) == 0 {
		return "", args, nil //no cluster status IDs found, return empty SQL stmt
	}

	return subQuery.String(), args, nil
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
	idColName, err := statusColHandler.ColumnName("ID")
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

	//query status entities (using sub-query in WHERE condition)
	q, err := db.NewQuery(i.Conn, clusterStatusEntity, i.Logger)
	if err != nil {
		return nil, err
	}

	clusterStatuses, err := q.Select().
		WhereIn("ID", fmt.Sprintf("SELECT %s FROM %s WHERE %s", idColName, clusterStatusEntity.Table(), sqlCond)).
		OrderBy(map[string]string{"ID": "DESC"}).
		GetMany()
	if err != nil {
		return nil, err
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
		clusterStatusEntity := clusterStatus.(*model.ClusterStatusEntity)
		var duration time.Duration
		if createdPrevStatus.IsZero() {
			duration = time.Since(clusterStatusEntity.Created)
		} else {
			duration = createdPrevStatus.Sub(clusterStatusEntity.Created)
		}

		statusChanges = append(statusChanges, &StatusChange{
			Status:   clusterStatusEntity,
			Duration: duration,
		})

		createdPrevStatus = clusterStatusEntity.Created
	}

	return statusChanges, nil
}

func (i *DefaultInventory) GetStatusIDsBlocksToDelete(statusCleanupBatchSize int) ([][]interface{}, error) {
	statusSelectQuery, err := db.NewQuery(i.Conn, &model.StatusCleanupEntity{}, i.Logger)
	if err != nil {
		return nil, err
	}
	statuses, err := statusSelectQuery.Select().GetMany()
	if err != nil {
		return nil, err
	}

	var statusIDs []interface{}
	for _, recon := range statuses {
		statusIDs = append(statusIDs, recon.(*model.StatusCleanupEntity).StatusID)
	}
	return repository.SplitSliceByBlockSize(statusIDs, statusCleanupBatchSize), nil
}

func (i *DefaultInventory) RemoveStatusesWithoutReconciliations(timeout time.Duration, statusCleanupBatchSize int) (int, error) {
	statusIDsBlocks, err := i.GetStatusIDsBlocksToDelete(statusCleanupBatchSize)
	if err != nil {
		return 0, err
	}
	totalDeleteCount := 0
	for _, statusIDsBlock := range statusIDsBlocks {
		dbOps := func(tx *db.TxConnection) (interface{}, error) {
			var args []interface{}
			var buffer bytes.Buffer

			for i, statusID := range statusIDsBlock {
				if buffer.Len() > 0 {
					buffer.WriteRune(',')
				}
				buffer.WriteString(fmt.Sprintf("$%d", i+1))
				args = append(args, statusID.(int64))
			}

			deleteQuery, err := db.NewQuery(tx, &model.ClusterStatusEntity{}, i.Logger)
			if err != nil {
				return 0, err
			}
			deletedRows, err := deleteQuery.
				Delete().
				WhereIn("ID", buffer.String(), args...).
				Exec()
			return int(deletedRows), err
		}
		delCnt, err := db.TransactionResult(i.Conn, dbOps, i.Logger)
		if err != nil {
			i.Logger.Error(fmt.Errorf("removal of config statuses without reconciliation failed during cluster entities cleanup, deleted count: %d %w", delCnt.(int), err))
		}
		totalDeleteCount += delCnt.(int)
		time.Sleep(timeout)
	}
	return totalDeleteCount, err
}

func (i *DefaultInventory) RemoveDeletedClustersOlderThan(deadline time.Time) (int, error) {
	dbOps := func(tx *db.TxConnection) (interface{}, error) {
		selectQuery, err := db.NewQuery(tx, &model.ClusterEntity{}, i.Logger)
		if err != nil {
			return 0, err
		}
		columnHandler, err := db.NewColumnHandler(&model.ClusterEntity{}, tx, i.Logger)
		if err != nil {
			return 0, err
		}
		createdColumn, err := columnHandler.ColumnName("Created")
		if err != nil {
			return 0, err
		}
		runtimeIDSelectQuery, err := selectQuery.SelectColumn("RuntimeID")
		if err != nil {
			return 0, err
		}
		runtimeIDSelectQuery.
			WhereRaw(fmt.Sprintf("%s<$%d", createdColumn, runtimeIDSelectQuery.NextPlaceholderCount()), deadline.Format("2006-01-02 15:04:05.000"))

		deleteQuery, err := db.NewQuery(tx, &model.ClusterEntity{}, i.Logger)
		if err != nil {
			return 0, err
		}
		whereCond := map[string]interface{}{
			"Deleted": true,
		}
		deletedRows, err := deleteQuery.
			Delete().
			WhereIn("RuntimeID", runtimeIDSelectQuery.String(), runtimeIDSelectQuery.GetArgs()...).
			Where(whereCond).
			Exec()
		return int(deletedRows), err
	}
	result, err := db.TransactionResult(i.Conn, dbOps, i.Logger)
	if err != nil {
		i.Logger.Error("Removal of deleted clusters failed during cluster entities cleanup", err)
	}
	return result.(int), err
}
