package cluster

import (
	uuid "github.com/google/uuid"
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

func (ci *Inventory) Add(cluster *model.Cluster) error {
	clusterID := uuid.New().String()
	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{
		ID:                 clusterID,
		Cluster:            cluster.Cluster,
		RuntimeName:        cluster.RuntimeInput.Name,
		RuntimeDescription: cluster.RuntimeInput.Description,
		GlobalAccountID:    cluster.Metadata.GlobalAccountID,
		SubAccountID:       cluster.Metadata.SubAccountID,
		ServiceID:          cluster.Metadata.ServiceID,
		ServicePlanID:      cluster.Metadata.ServicePlanID,
		ShootName:          cluster.Metadata.ShootName,
		InstanceID:         cluster.Metadata.InstanceID,
		Created:            time.Time{},
	})
	if err != nil {
		return err
	}
	err = q.Insert().Exec()
	if err != nil {
		return err
	}

	configurationID := uuid.New().String()
	q, err = db.NewQuery(ci.Conn, &model.ConfigurationEntity{
		ID:          configurationID,
		ClusterID:   clusterID,
		KymaVersion: cluster.KymaConfig.Version,
		KymaProfile: cluster.KymaConfig.Profile,
		Created:     time.Time{},
	})
	if err != nil {
		return err
	}
	err = q.Insert().Exec()
	if err != nil {
		return err
	}

	if cluster.KymaConfig.Administrators != nil {
		for _, admin := range cluster.KymaConfig.Administrators {
			q, err := db.NewQuery(ci.Conn, &model.ClusterAdministratorEntity{
				ID:              uuid.New().String(),
				ConfigurationID: configurationID,
				UserId:          admin,
				Created:         time.Time{},
			})
			if err != nil {
				return err
			}
			err = q.Insert().Exec()
			if err != nil {
				return err
			}
		}
	}

	for _, component := range cluster.KymaConfig.Components {
		componentID := uuid.New().String()
		q, err = db.NewQuery(ci.Conn, &model.ComponentEntity{
			ID:              componentID,
			ConfigurationID: configurationID,
			Component:       component.Component,
			Namespace:       component.Namespace,
			Created:         time.Time{},
		})
		if err != nil {
			return err
		}
		err = q.Insert().Exec()
		if err != nil {
			return err
		}

		for _, config := range component.Configuration {
			q, err = db.NewQuery(ci.Conn, &model.ComponentConfigurationEntity{
				ID:          uuid.New().String(),
				ComponentID: componentID,
				Key:         config.Key,
				Value:       config.Value,
				Secret:      config.Secret,
				Created:     time.Time{},
			})
			if err != nil {
				return err
			}
			err = q.Insert().Exec()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ci *Inventory) Delete(cluster string) error {
	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{})
	if err != nil {
		return err
	}
	_, err = q.Delete().Where(map[string]interface{}{"Cluster": cluster}).Exec()
	if err != nil {
		return err
	}
	return nil
}

//TODO
//func (ci *Inventory) GetByStatuses(statuses []model.ClusterStatus) ([]*model.ClusterEntity, error) {
//	q, err := db.NewQuery(ci.Conn, &model.ClusterEntity{})
//	if err != nil {
//		return nil, err
//	}
//	var values []string
//	for _, status := range statuses {
//		values = append(values, "'"+string(status)+"'")
//	}
//	entities, err := q.Select().WhereIn("Status", strings.Join(values, ","), nil).GetMany()
//	if err != nil {
//		return nil, err
//	}
//	result := []*model.ClusterEntity{}
//	for _, entity := range entities {
//		result = append(result, entity.(*model.ClusterEntity))
//	}
//	return result, nil
//}
