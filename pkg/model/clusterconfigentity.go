package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

const (
	CRDComponent            = "CRDs"
	tblConfiguration string = "inventory_cluster_configs"
)

type ReconciliationSequence struct {
	Queue [][]*keb.Component
}

type ClusterConfigurationEntity struct {
	Version        int64            `db:"readOnly"`
	Cluster        string           `db:"notNull"`
	ClusterVersion int64            `db:"notNull"`
	KymaVersion    string           `db:"notNull"`
	KymaProfile    string           `db:""`
	Components     *[]keb.Component `db:"notNull"`
	Administrators *[]string
	Contract       int64     `db:"notNull"`
	Deleted        bool      `db:"notNull"`
	Created        time.Time `db:"readOnly"`
}

func (c *ClusterConfigurationEntity) String() string {
	return fmt.Sprintf("ClusterConfigurationEntity [Version=%d,Cluster=%s,ClusterVersion=%d]",
		c.Version, c.Cluster, c.ClusterVersion)
}

func (c *ClusterConfigurationEntity) New() db.DatabaseEntity {
	return &ClusterConfigurationEntity{}
}

func (c *ClusterConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)

	marshaller.AddUnmarshaller("Components", func(value interface{}) (interface{}, error) {
		var mapConfig *[]keb.Component
		err := json.Unmarshal([]byte(value.(string)), &mapConfig)
		return mapConfig, err
	})
	marshaller.AddUnmarshaller("Administrators", func(value interface{}) (interface{}, error) {
		var mapConfig *[]string
		err := json.Unmarshal([]byte(value.(string)), &mapConfig)
		return mapConfig, err
	})

	marshaller.AddMarshaller("Components", convertInterfaceToString)
	marshaller.AddMarshaller("Administrators", convertInterfaceToString)
	return marshaller
}

func (c *ClusterConfigurationEntity) Table() string {
	return tblConfiguration
}

func (c *ClusterConfigurationEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherClProp, ok := other.(*ClusterConfigurationEntity)
	if ok {
		return c.Cluster == otherClProp.Cluster &&
			c.ClusterVersion == otherClProp.ClusterVersion &&
			c.KymaVersion == otherClProp.KymaVersion &&
			c.KymaProfile == otherClProp.KymaProfile &&
			c.Components == otherClProp.Components &&
			c.Administrators == otherClProp.Administrators &&
			c.Contract == otherClProp.Contract
	}
	return false
}

func (c *ClusterConfigurationEntity) GetComponent(component string) (*keb.Component, error) {
	reconSeq, err := c.GetReconciliationSequence(nil)
	if err != nil {
		return nil, err
	}
	for _, compGroup := range reconSeq.Queue {
		for _, comp := range compGroup {
			if comp.Component == component {
				return comp, nil
			}
		}
	}
	return nil, nil
}

func (c *ClusterConfigurationEntity) GetReconciliationSequence(preComponents []string) (*ReconciliationSequence, error) {
	//group components depending on their reconciliation order
	sequence := &ReconciliationSequence{}
	sequence.Queue = append(sequence.Queue, []*keb.Component{
		{Component: CRDComponent, Namespace: "default"},
	})

	var inParallel []*keb.Component
	if c.Components != nil {
		for _, component := range *c.Components {
			if contains(preComponents, component.Component) {
				sequence.Queue = append(sequence.Queue, []*keb.Component{
					&component,
				})
			} else {
				comp := keb.Component{
					URL:           component.URL,
					Component:     component.Component,
					Configuration: component.Configuration,
					Namespace:     component.Namespace,
					Version:       component.Version,
				}
				inParallel = append(inParallel, &comp)
			}
		}
	}
	if len(inParallel) > 0 {
		sequence.Queue = append(sequence.Queue, inParallel)
	}

	return sequence, nil
}

func contains(items []string, item string) bool {
	for i := range items {
		if item == items[i] {
			return true
		}
	}
	return false
}
