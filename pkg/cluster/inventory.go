package cluster

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/repository"
)

const tblCache string = "cluster"

// TODO
type Cluster struct {
	ID          int64 `db:"readOnly"`
	Name        string
	KymaVersion string
	State       string
}

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

func (ce *Cluster) String() string {
	return fmt.Sprintf("Cluster [Name=%d,KymaVersion=%s,State=%s]",
		ce.Name, ce.KymaVersion, ce.State)
}

func (ce *Cluster) New() db.DatabaseEntity {
	return &Cluster{}
}

func (ce *Cluster) Table() string {
	return tblCache
}

func (ce *Cluster) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherEntry, ok := other.(*Cluster)
	if ok {
		return ce.Name == otherEntry.Name &&
			ce.KymaVersion == otherEntry.KymaVersion &&
			ce.State == otherEntry.State
	}
	return false
}

func (ce *Cluster) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&ce)
	return marshaller
}

func (ci *Inventory) All() ([]*Cluster, error) {
	q, err := db.NewQuery(ci.Conn, &Cluster{})
	if err != nil {
		return nil, err
	}
	entities, err := q.Select().GetMany()
	if err != nil {
		return nil, err
	}
	result := []*Cluster{}
	for _, entity := range entities {
		result = append(result, entity.(*Cluster))
	}
	return result, nil
}

func (ci *Inventory) Get(clusterName string) (*Cluster, error) {
	q, err := db.NewQuery(ci.Conn, &Cluster{})
	if err != nil {
		return nil, err
	}
	entity, err := q.Select().Where(map[string]interface{}{"Name": clusterName}).GetOne()
	if err != nil {
		return nil, err
	}
	return entity.(*Cluster), nil
}

func (ci *Inventory) Add(cluster *Cluster) error {
	q, err := db.NewQuery(ci.Conn, &Cluster{
		Name:        cluster.Name,
		KymaVersion: cluster.KymaVersion,
		State:       cluster.State,
	})
	if err != nil {
		return err
	}
	err = q.Insert().Exec()
	if err != nil {
		return err
	}
	return nil
}

func (ci *Inventory) Delete(clusterName string) error {
	q, err := db.NewQuery(ci.Conn, &Cluster{})
	if err != nil {
		return err
	}
	_, err = q.Delete().Where(map[string]interface{}{"Name": clusterName}).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (ci *Inventory) Update(fields []string, clusterName string) error {
	q, err := db.NewQuery(ci.Conn, &Cluster{
		Name: clusterName,
	})
	if err != nil {
		return err
	}
	err = q.Update(fields).Exec()

	if err != nil {
		return err
	}
	return nil
}
