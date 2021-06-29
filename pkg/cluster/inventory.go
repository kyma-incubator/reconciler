package cluster

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/pkg/errors"
)

type Inventory interface {
	CreateOrUpdate(cluster *Cluster) (*ClusterState, error)
	UpdateStatus(clusterState *ClusterState) error
	Delete(cluster string) error
	Get(cluster string, configVersion int64) (*ClusterState, error)
	ClustersToReconcile() ([]*ClusterState, error)
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

func (i *DefaultInventory) CreateOrUpdate(cluster *Cluster) (*ClusterState, error) {
	dbOps := func() (interface{}, error) {
		clusterEntity, err := i.createCluster(cluster)
		if err != nil {
			return nil, err
		}
		clusterConfigurationEntity, err := i.createConfiguration(cluster, clusterEntity)
		if err != nil {
			return nil, err
		}
		clusterStatusEntity, err := i.createStatus(clusterConfigurationEntity)
		if err != nil {
			return nil, err
		}
		return &ClusterState{
			Cluster:       clusterEntity,
			Configuration: clusterConfigurationEntity,
			Status:        clusterStatusEntity,
		}, nil
	}
	clusterState, err := db.TransactionResult(i.Conn, dbOps, i.Logger)
	if err != nil {
		return nil, err
	}
	return clusterState.(*ClusterState), nil
}

func (i *DefaultInventory) createCluster(cluster *Cluster) (*model.ClusterEntity, error) {
	metadata, err := json.Marshal(cluster.Metadata)
	if err != nil {
		return nil, err
	}
	clusterEntity := &model.ClusterEntity{
		Cluster:            cluster.Cluster,
		RuntimeName:        cluster.RuntimeInput.Name,
		RuntimeDescription: cluster.RuntimeInput.Description,
		Metadata:           string(metadata),
	}
	q, err := db.NewQuery(i.Conn, clusterEntity)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}
	return clusterEntity, nil
}

func (i *DefaultInventory) createConfiguration(cluster *Cluster, clusterEntity *model.ClusterEntity) (*model.ClusterConfigurationEntity, error) {
	components, err := json.Marshal(cluster.KymaConfig.Components)
	if err != nil {
		return nil, err
	}
	administrators, err := json.Marshal(cluster.KymaConfig.Administrators)
	if err != nil {
		return nil, err
	}
	configEntity := &model.ClusterConfigurationEntity{
		Cluster:        clusterEntity.Cluster,
		ClusterVersion: clusterEntity.Version,
		KymaVersion:    cluster.KymaConfig.Version,
		KymaProfile:    cluster.KymaConfig.Profile,
		Components:     string(components),
		Administrators: string(administrators),
	}
	q, err := db.NewQuery(i.Conn, configEntity)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}
	return configEntity, nil
}

func (i *DefaultInventory) createStatus(configEntity *model.ClusterConfigurationEntity) (*model.ClusterStatusEntity, error) {
	statusEntity := &model.ClusterStatusEntity{
		ConfigVersion: configEntity.Version,
		Status:        model.ReconcilePending,
	}
	q, err := db.NewQuery(i.Conn, statusEntity)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}
	return statusEntity, nil
}

func (i *DefaultInventory) UpdateStatus(clusterState *ClusterState) error {
	configEntity := &model.ClusterConfigurationEntity{}
	q, err := db.NewQuery(i.Conn, configEntity)
	if err != nil {
		return err
	}
	return q.Insert().Exec()
}

func (i *DefaultInventory) Delete(cluster string) error {
	//TBC: do we delete a cluster in the DB or flag it as deleted?
	return fmt.Errorf("Method not supported yet")
}

func (i *DefaultInventory) Get(cluster string, configVersion int64) (*ClusterState, error) {
	statusEntity, err := i.latestStatus(configVersion)
	if err != nil {
		return nil, err
	}
	configEntity, err := i.config(cluster, statusEntity.ConfigVersion)
	if err != nil {
		return nil, err
	}
	clusterEntity, err := i.cluster(configEntity.ClusterVersion)
	if err != nil {
		return nil, err
	}
	return &ClusterState{
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
	statusEntity, err := q.Select().
		Where(map[string]interface{}{"ConfigVersion": configVersion}).
		OrderBy(map[string]string{"ID": "desc"}).
		GetOne()
	if err != nil {
		return nil, errors.Wrap(err,
			fmt.Sprintf("No status entities found for cluster configuration with version '%d'", configVersion))
	}
	return statusEntity.(*model.ClusterStatusEntity), nil
}

func (i *DefaultInventory) config(cluster string, configVersion int64) (*model.ClusterConfigurationEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterConfigurationEntity{})
	if err != nil {
		return nil, err
	}
	configEntity, err := q.Select().
		Where(map[string]interface{}{
			"Version": configVersion,
			"Cluster": cluster,
		}).
		GetOne()
	if err != nil {
		return nil, errors.Wrap(err,
			fmt.Sprintf("Cluster configuration '%d' does not exist for cluster '%s' ", configVersion, cluster))
	}
	return configEntity.(*model.ClusterConfigurationEntity), nil
}

func (i *DefaultInventory) cluster(clusterVersion int64) (*model.ClusterEntity, error) {
	q, err := db.NewQuery(i.Conn, &model.ClusterEntity{})
	if err != nil {
		return nil, err
	}
	clusterEntity, err := q.Select().
		Where(map[string]interface{}{
			"Version": clusterVersion,
		}).
		GetOne()
	if err != nil {
		return nil, errors.Wrap(err,
			fmt.Sprintf("No cluster found using lusterVersion '%d'", clusterVersion))
	}
	return clusterEntity.(*model.ClusterEntity), nil
}

func (i *DefaultInventory) ClustersToReconcile() ([]*ClusterState, error) {
	return nil, nil
}
