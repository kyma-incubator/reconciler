package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
)

type Inventory interface {
	CreateOrUpdate(contractVersion int64, cluster *keb.Cluster) (*State, error)
	UpdateStatus(State *State, status model.Status) (*State, error)
	Delete(cluster string) error
	Get(cluster string, configVersion int64) (*State, error)
	GetLatest(cluster string) (*State, error)
	StatusChanges(cluster string, offset time.Duration) ([]*StatusChange, error)
	ClustersToReconcile(reconcileInterval time.Duration) ([]*State, error)
	ClustersNotReady() ([]*State, error)
}

type DefaultInventory struct {
	*repository.Repository
	metricsCollector
}

type metricsCollector interface {
	OnClusterStateUpdate(state *State) error
}

func NewInventory(dbFac db.ConnectionFactory, debug bool, collector metricsCollector) (Inventory, error) {
	repo, err := repository.NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &DefaultInventory{repo, collector}, nil
}

func (i *DefaultInventory) CreateOrUpdate(contractVersion int64, cluster *keb.Cluster) (*State, error) {
	dbOps := func() (interface{}, error) {
		clusterEntity, err := i.createCluster(contractVersion, cluster)
		if err != nil {
			return nil, err
		}
		clusterConfigurationEntity, err := i.createConfiguration(contractVersion, cluster, clusterEntity)
		if err != nil {
			return nil, err
		}
		clusterStatusEntity, err := i.createStatus(clusterConfigurationEntity, model.ReconcilePending)
		if err != nil {
			return nil, err
		}
		return &State{
			Cluster:       clusterEntity,
			Configuration: clusterConfigurationEntity,
			Status:        clusterStatusEntity,
		}, nil
	}
	stateEntity, err := db.TransactionResult(i.Conn, dbOps, i.Logger)
	if err != nil {
		return nil, err
	}
	err = i.metricsCollector.OnClusterStateUpdate(stateEntity.(*State))
	if err != nil {
		return nil, err
	}
	return stateEntity.(*State), nil
}

func (i *DefaultInventory) createCluster(contractVersion int64, cluster *keb.Cluster) (*model.ClusterEntity, error) {
	metadata, err := json.Marshal(cluster.Metadata)
	if err != nil {
		return nil, err
	}
	runtime, err := json.Marshal(cluster.RuntimeInput)
	if err != nil {
		return nil, err
	}

	newClusterEntity := &model.ClusterEntity{
		Cluster:    cluster.Cluster,
		Runtime:    string(runtime),
		Metadata:   string(metadata),
		Kubeconfig: "fixme", //TODO: use correct model field as soon as kubeconfig is provided
		Contract:   contractVersion,
	}

	//check if a new version is required
	oldClusterEntity, err := i.latestCluster(cluster.Cluster)
	if err == nil {
		if oldClusterEntity.Equal(newClusterEntity) { //reuse existing cluster entity
			i.Logger.Debugf("No differences found for cluster '%s': not creating new database entity", cluster.Cluster)
			return oldClusterEntity, nil
		}
	} else if !repository.IsNotFoundError(err) {
		//unexpected error
		return nil, err
	}

	//create new version
	q, err := db.NewQuery(i.Conn, newClusterEntity)
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
	components, err := json.Marshal(cluster.KymaConfig.Components)
	if err != nil {
		return nil, err
	}
	administrators, err := json.Marshal(cluster.KymaConfig.Administrators)
	if err != nil {
		return nil, err
	}
	newConfigEntity := &model.ClusterConfigurationEntity{
		Cluster:        clusterEntity.Cluster,
		ClusterVersion: clusterEntity.Version,
		KymaVersion:    cluster.KymaConfig.Version,
		KymaProfile:    cluster.KymaConfig.Profile,
		Components:     string(components),
		Administrators: string(administrators),
		Contract:       contractVersion,
	}

	//check if a new version is required
	oldConfigEntity, err := i.latestConfig(clusterEntity.Version)
	if err == nil {
		if oldConfigEntity.Equal(newConfigEntity) { //reuse existing config entity
			i.Logger.Debugf("No differences found for configuration of cluster '%s': not creating new database entity", cluster.Cluster)
			return oldConfigEntity, nil
		}
	} else if !repository.IsNotFoundError(err) {
		//unexpected error
		return nil, err
	}

	//create new version
	q, err := db.NewQuery(i.Conn, newConfigEntity)
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
		Cluster:        configEntity.Cluster,
		ClusterVersion: configEntity.ClusterVersion,
		ConfigVersion:  configEntity.Version,
		Status:         status,
	}

	//check if a new version is required
	oldStatusEntity, err := i.latestStatus(configEntity.Version)
	if err == nil {
		if oldStatusEntity.Equal(newStatusEntity) { //reuse existing status entity
			i.Logger.Debugf("No differences found for status of cluster '%s': not creating new database entity", configEntity.Cluster)
			return oldStatusEntity, nil
		}
	} else if !repository.IsNotFoundError(err) {
		//unexpected error
		return nil, err
	}

	//create new status
	q, err := db.NewQuery(i.Conn, newStatusEntity)
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

func (i *DefaultInventory) Delete(cluster string) error {
	dbOps := func() error {
		newClusterName := fmt.Sprintf("deleted_%d_%s", time.Now().Unix(), cluster)
		updateSQLTpl := "UPDATE %s SET %s=$1, %s='TRUE' WHERE %s=$2 OR %s=$3" //OR condition required for Postgres: new cluster-name is automatically cascaded to config-status table

		//update name of all cluster entities
		clusterEntity := &model.ClusterEntity{}
		clusterColHandler, err := db.NewColumnHandler(clusterEntity)
		if err != nil {
			return err
		}
		clusterColName, err := clusterColHandler.ColumnName("Cluster")
		if err != nil {
			return err
		}
		clusterDelColName, err := clusterColHandler.ColumnName("Deleted")
		if err != nil {
			return err
		}
		clusterUpdateSQL := fmt.Sprintf(updateSQLTpl, clusterEntity.Table(), clusterColName, clusterDelColName, clusterColName, clusterColName)
		if _, err := i.Conn.Exec(clusterUpdateSQL, newClusterName, cluster, newClusterName); err != nil {
			return err
		}

		//update cluster-name of all referenced cluster-config entities
		configEntity := &model.ClusterConfigurationEntity{}
		configColHandler, err := db.NewColumnHandler(configEntity)
		if err != nil {
			return err
		}
		configClusterColName, err := configColHandler.ColumnName("Cluster")
		if err != nil {
			return err
		}
		configDelColName, err := configColHandler.ColumnName("Deleted")
		if err != nil {
			return err
		}
		configUpdateSQL := fmt.Sprintf(updateSQLTpl, configEntity.Table(), configClusterColName, configDelColName, configClusterColName, configClusterColName)
		if _, err := i.Conn.Exec(configUpdateSQL, newClusterName, cluster, newClusterName); err != nil {
			return err
		}

		//done
		return nil
	}
	return db.Transaction(i.Conn, dbOps, i.Logger)
}

func (i *DefaultInventory) Get(cluster string, configVersion int64) (*State, error) {
	configEntity, err := i.config(cluster, configVersion)
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

func (i *DefaultInventory) GetLatest(cluster string) (*State, error) {
	clusterEntity, err := i.latestCluster(cluster)
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

func (i *DefaultInventory) latestStatus(configVersion int64) (*model.ClusterStatusEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterStatusEntity{})
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
		return nil, i.NewNotFoundError(err, statusEntity, whereCond)
	}
	return statusEntity.(*model.ClusterStatusEntity), nil
}

func (i *DefaultInventory) config(cluster string, configVersion int64) (*model.ClusterConfigurationEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterConfigurationEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"Version": configVersion,
		"Cluster": cluster,
	}
	configEntity, err := q.Select().
		Where(whereCond).
		GetOne()
	if err != nil {
		return nil, i.NewNotFoundError(err, configEntity, whereCond)
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) latestConfig(clusterVersion int64) (*model.ClusterConfigurationEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterConfigurationEntity{})
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
		return nil, i.NewNotFoundError(err, configEntity, whereCond)
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) cluster(clusterVersion int64) (*model.ClusterEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterEntity{})
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
		return nil, i.NewNotFoundError(err, clusterEntity, whereCond)
	}
	return clusterEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) latestCluster(cluster string) (*model.ClusterEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterEntity{})
	if err != nil {
		return nil, err
	}
	whereCond := map[string]interface{}{
		"Cluster": cluster,
		"Deleted": false,
	}
	clusterEntity, err := q.Select().
		Where(whereCond).
		OrderBy(map[string]string{
			"Version": "desc",
		}).
		GetOne()
	if err != nil {
		return nil, i.NewNotFoundError(err, clusterEntity, whereCond)
	}
	return clusterEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) ClustersToReconcile(reconcileInterval time.Duration) ([]*State, error) {
	filters := []statusSQLFilter{}
	if reconcileInterval > 0 {
		filters = append(filters, &reconcileIntervalFilter{
			reconcileInterval: reconcileInterval,
		})
	}
	filters = append(filters, &statusFilter{
		allowedStatuses: []model.Status{model.ReconcilePending, model.ReconcileFailed},
	})
	return i.filterClusters(filters...)
}

func (i *DefaultInventory) ClustersNotReady() ([]*State, error) {
	statusFilter := &statusFilter{
		allowedStatuses: []model.Status{model.Reconciling, model.ReconcileFailed, model.Error},
	}
	return i.filterClusters(statusFilter)
}

func (i *DefaultInventory) filterClusters(filters ...statusSQLFilter) ([]*State, error) {
	//get DDL for sub-query
	clusterStatus := &model.ClusterStatusEntity{}
	statusColHandler, err := db.NewColumnHandler(clusterStatus)
	if err != nil {
		return nil, err
	}
	idColName, err := statusColHandler.ColumnName("ID")
	if err != nil {
		return nil, err
	}
	clusterColName, err := statusColHandler.ColumnName("Cluster")
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

	//get cluster configurations of all "not-ready" clusters
	q, err := db.NewQuery(i.Conn, &model.ClusterConfigurationEntity{})
	if err != nil {
		return nil, err
	}

	/*
		select config_version from inventory_cluster_config_statuses where id in (
			select maxid from (
				select max(cluster_version) as maxcfg, max(id) as maxid from inventory_cluster_config_statuses group by cluster
			) as maxid
		) and (status in ('xxx', 'yyy') or (status = 'ready' AND created >=  NOW() - INTERVAL '12345 SECOND'))
	*/
	var sqlFilterStmt bytes.Buffer
	if len(filters) == 0 {
		sqlFilterStmt.WriteString("1=1") //if no filters are provided, use 1=1 as placeholder to ensure valid SQL query
	}
	for _, filter := range filters {
		sqlCond, err := filter.Filter(i.Conn.Type(), statusColHandler)
		if err != nil {
			return nil, err
		}
		if sqlFilterStmt.Len() > 0 {
			sqlFilterStmt.WriteString(" OR ")
		}
		sqlFilterStmt.WriteRune('(')
		sqlFilterStmt.WriteString(sqlCond)
		sqlFilterStmt.WriteRune(')')
	}

	clusterConfigs, err := q.Select().
		WhereIn("Version",
			fmt.Sprintf(`SELECT %s FROM %s WHERE %s IN (
							SELECT maxid FROM (
								SELECT MAX(%s) AS maxcfg, MAX(%s) AS maxid FROM %s GROUP BY %s
							) AS maxid
						) AND (%s)`,
				configVersionColName, clusterStatus.Table(), idColName,
				clusterVersionColName, idColName, clusterStatus.Table(), clusterColName,
				sqlFilterStmt.String())).
		Where(map[string]interface{}{
			"Deleted": false,
		}).
		GetMany()
	if err != nil {
		return nil, err
	}

	//retreive clusters which require a reconciliation
	result := []*State{}
	for _, clusterConfig := range clusterConfigs {
		clusterConfigEntity := clusterConfig.(*model.ClusterConfigurationEntity)
		state, err := i.Get(clusterConfigEntity.Cluster, clusterConfigEntity.Version)
		if err != nil {
			return nil, err
		}
		result = append(result, state)
	}

	return result, nil
}

func (i *DefaultInventory) StatusChanges(cluster string, offset time.Duration) ([]*StatusChange, error) {
	statusColHandler, err := db.NewColumnHandler(&model.ClusterStatusEntity{})
	if err != nil {
		return nil, err
	}
	statusColName, err := statusColHandler.ColumnName("Status")
	if err != nil {
		return nil, err
	}
	createdColName, err := statusColHandler.ColumnName("Created")
	if err != nil {
		return nil, err
	}

	filter := createdIntervalFilter{
		interval: offset,
		cluster:  cluster,
	}
	sqlCond, err := filter.Filter(i.Conn.Type(), statusColHandler)
	if err != nil {
		return nil, err
	}
	rows, err := i.Conn.Query(fmt.Sprintf("SELECT %s, %s FROM %s WHERE %s ORDER BY %s DESC", statusColName, createdColName, (&model.ClusterStatusEntity{}).Table(), sqlCond, createdColName))
	if err != nil {
		return nil, err
	}
	var statusChanges []*StatusChange
	var createdPrevStatus time.Time
	var createdCurrStatus time.Time
	for rows.Next() {
		var status *model.Status
		if err := rows.Scan(&status, &createdCurrStatus); err != nil {
			return nil, err
		}
		var duration string
		if createdPrevStatus.IsZero() {
			duration = time.Since(createdCurrStatus).String()
		} else {
			duration = createdPrevStatus.Sub(createdCurrStatus).String()
		}
		statusChanges = append(statusChanges, &StatusChange{
			Status:   status,
			Duration: duration,
		})
		createdPrevStatus = createdCurrStatus
	}
	return statusChanges, nil
}
