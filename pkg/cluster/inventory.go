package cluster

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"time"
)

type Inventory struct {
	*repository.Repository
}

func NewRepository(dbFac db.ConnectionFactory, debug bool) (*Inventory, error) {
	repo, err := repository.NewRepository(dbFac, debug)
	if err != nil {
		return nil, err
	}
	return &Inventory{repo}, nil
}

func (ci *Inventory) All() ([]*model.ClusterEntity, error) {
	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{})
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().GetMany()
	if err != nil {
		return nil, err
	}
	result := []*model.ClusterEntity{}
	for _, entity := range entities {
		result = append(result, entity.(*model.ClusterEntity))
	}
	return result, nil
}

func (ci *Inventory) Get(clusterName string) (*model.ClusterEntity, error) {
	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{})
	if err != nil {
		return nil, err
	}
	entity, err := q.Select().Where(map[string]interface{}{"Name": clusterName}).GetOne()
	if err != nil {
		return nil, err
	}
	return entity.(*model.ClusterEntity), nil
}

func (ci *Inventory) Add(cluster *model.Cluster) (*model.ClusterEntity, error) {
	metadata, err := json.Marshal(cluster.Metadata)
	if err != nil {
		return nil, err
	}
	clusterEntity := &model.ClusterEntity{
		Cluster:            cluster.Cluster,
		RuntimeName:        cluster.RuntimeInput.Name,
		RuntimeDescription: cluster.RuntimeInput.Description,
		Metadata:           string(metadata),
		Created:            time.Time{},
	}

	q, err := db.NewQuery(ci.Conn, clusterEntity)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}

	components, err := json.Marshal(cluster.KymaConfig.Components)
	if err != nil {
		return nil, err
	}
	administrators, err := json.Marshal(cluster.KymaConfig.Administrators)
	if err != nil {
		return nil, err
	}
	configurationEntity := &model.ConfigurationEntity{
		ClusterID:      clusterEntity.ID,
		KymaVersion:    cluster.KymaConfig.Version,
		KymaProfile:    cluster.KymaConfig.Profile,
		Components:     string(components),
		Administrators: string(administrators),
		Created:        time.Time{},
	}

	q, err = db.NewQuery(ci.Conn, configurationEntity)
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}

	q, err = db.NewQuery(ci.Conn, &model.StatusEntity{
		ConfigurationID: configurationEntity.ID,
		Status:          model.ReconcilePending,
		Created:         time.Time{},
	})
	if err != nil {
		return nil, err
	}
	err = q.Insert().Exec()
	if err != nil {
		return nil, err
	}

	return clusterEntity, nil
}

//TODO do we have to delete entries in DB or just set the state to deleted
func (ci *Inventory) Delete(cluster string) error {
	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{})
	if err != nil {
		return err
	}
	_, err = q.Delete().Where(map[string]interface{}{"ID": cluster}).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (ci *Inventory) GetClusterStatus(clusterId int) (*model.StatusEntity, error) {
	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{})
	if err != nil {
		return nil, err
	}
	cluster, err := q.Select().Where(map[string]interface{}{"ID": clusterId}).GetOne()
	if err != nil {
		return nil, err
	}
	q, err = db.NewQuery(ci.Conn, &model.ConfigurationEntity{})
	config, err := q.Select().
		Where(map[string]interface{}{"ClusterID": cluster.(*model.ClusterEntity).ID}).
		OrderBy(map[string]string{"ID": "DESC"}).
		GetOne()
	if err != nil {
		return nil, err
	}

	q, err = db.NewQuery(ci.Conn, &model.StatusEntity{})
	status, err := q.Select().
		Where(map[string]interface{}{"ConfigurationID": config.(*model.ConfigurationEntity).ID}).
		OrderBy(map[string]string{"ID": "DESC"}).
		GetOne()
	if err != nil {
		return nil, err
	}
	return status.(*model.StatusEntity), nil
}
