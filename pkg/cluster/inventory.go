package cluster

import (
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
	ClustersToReconcile() ([]*State, error)
	ClustersNotReady() ([]*State, error)
}

type DefaultInventory struct {
	*repository.Repository
	reconcileInterval time.Duration
}

func NewInventory(dbFac db.ConnectionFactory, debug bool) (Inventory, error) {
	repo, err := repository.NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &DefaultInventory{repo, 0}, nil
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
		Cluster:  cluster.Cluster,
		Runtime:  string(runtime),
		Metadata: string(metadata),
		Contract: contractVersion,
	}

	//check if a new version is required
	oldClusterEntity, err := i.latestCluster(cluster.Cluster)
	if err == nil {
		if oldClusterEntity.Equal(newClusterEntity) { //reuse existing cluster entity
			i.Logger.Debug(fmt.Sprintf("No differences found for cluster '%s': not creating new database entity", cluster.Cluster))
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
			i.Logger.Debug(
				fmt.Sprintf("No differences found for configuration of cluster '%s': not creating new database entity", cluster.Cluster))
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
		ConfigVersion: configEntity.Version,
		Status:        status,
	}

	//check if a new version is required
	oldStatusEntity, err := i.latestStatus(configEntity.Version)
	if err == nil {
		if oldStatusEntity.Equal(newStatusEntity) { //reuse existing status entity
			i.Logger.Debug(
				fmt.Sprintf("No differences found for status of cluster '%s': not creating new database entity", configEntity.Cluster))
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
	return state, nil
}

func (i *DefaultInventory) Delete(cluster string) error {
	dbOps := func() error {
		newClusterName := fmt.Sprintf("deleted_%d_%s", time.Now().Unix(), cluster)
		updateSQLTpl := "UPDATE %s SET %s=$1 WHERE %s=$2"

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
		clusterUpdateSQL := fmt.Sprintf(updateSQLTpl, clusterEntity.Table(), clusterColName, clusterColName)
		if _, err := i.Conn.Exec(clusterUpdateSQL, newClusterName, cluster); err != nil {
			return err
		}

		//update cluster-name of all referenced cluster-config entities
		configEntity := &model.ClusterConfigurationEntity{}
		configColHandler, err := db.NewColumnHandler(configEntity)
		if err != nil {
			return err
		}
		configColumnName, err := configColHandler.ColumnName("Cluster")
		if err != nil {
			return err
		}
		configUpdateSQL := fmt.Sprintf(updateSQLTpl, configEntity.Table(), configColumnName, configColumnName)
		if _, err := i.Conn.Exec(configUpdateSQL, newClusterName, cluster); err != nil {
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

func (i *DefaultInventory) ClustersToReconcile() ([]*State, error) {
	return nil, fmt.Errorf("Method not implemented yet")
}

func (i *DefaultInventory) ClustersNotReady() ([]*State, error) {
	return nil, fmt.Errorf("Method not implemented yet")
}
