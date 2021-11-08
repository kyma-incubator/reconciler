package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

const (
	CRDComponent            = "CRDs"
	tblConfiguration string = "inventory_cluster_configs"
)

var crdComponent = &keb.Component{Component: CRDComponent, Namespace: "default"}

type ReconciliationSequence struct {
	Queue [][]*keb.Component
}

type ClusterConfigurationEntity struct {
	Version        int64            `db:"readOnly"`
	RuntimeID      string           `db:"notNull"`
	ClusterVersion int64            `db:"notNull"` // Cluster entity primary key
	KymaVersion    string           `db:"notNull"`
	KymaProfile    string           `db:""`
	Components     []*keb.Component `db:"notNull"`
	Administrators []string
	Contract       int64     `db:"notNull"`
	Deleted        bool      `db:"notNull"`
	Created        time.Time `db:"readOnly"`
}

func (c *ClusterConfigurationEntity) String() string {
	return fmt.Sprintf("ClusterConfigurationEntity [Version=%d,RuntimeID=%s,ClusterVersion=%d]",
		c.Version, c.RuntimeID, c.ClusterVersion)
}

func (c *ClusterConfigurationEntity) New() db.DatabaseEntity {
	return &ClusterConfigurationEntity{}
}

func (c *ClusterConfigurationEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&c)
	marshaller.AddUnmarshaller("Created", convertTimestampToTime)

	marshaller.AddUnmarshaller("Components", func(value interface{}) (interface{}, error) {
		var comps []keb.Component
		err := json.Unmarshal([]byte(value.(string)), &comps)
		return func() []*keb.Component {
			var result []*keb.Component
			for idx := range comps {
				result = append(result, &comps[idx])
			}
			return result
		}(), err
	})
	marshaller.AddUnmarshaller("Administrators", func(value interface{}) (interface{}, error) {
		var result []string
		err := json.Unmarshal([]byte(value.(string)), &result)
		return result, err
	})

	marshaller.AddMarshaller("Components", convertInterfaceToJSONString)
	marshaller.AddMarshaller("Administrators", convertInterfaceToJSONString)
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
		return c.RuntimeID == otherClProp.RuntimeID &&
			c.ClusterVersion == otherClProp.ClusterVersion &&
			c.KymaVersion == otherClProp.KymaVersion &&
			c.KymaProfile == otherClProp.KymaProfile &&
			reflect.DeepEqual(c.Components, otherClProp.Components) &&
			reflect.DeepEqual(c.Administrators, otherClProp.Administrators) &&
			c.Contract == otherClProp.Contract
	}
	return false
}

func (c *ClusterConfigurationEntity) GetComponent(component string) *keb.Component {
	if component == CRDComponent { //CRD is an artificial component which doesn't exist in the component list of any cluster
		return crdComponent
	}
	for _, comp := range c.Components {
		if comp.Component == component {
			return comp
		}
	}
	return nil
}

func (c *ClusterConfigurationEntity) GetReconciliationSequence(preComponents []string) *ReconciliationSequence {
	//group components depending on their reconciliation order
	sequence := &ReconciliationSequence{}
	sequence.Queue = append(sequence.Queue, []*keb.Component{
		crdComponent,
	})

	var inParallel []*keb.Component
	for i := range c.Components {
		if contains(preComponents, c.Components[i].Component) {
			sequence.Queue = append(sequence.Queue, []*keb.Component{
				c.Components[i],
			})
		} else {
			inParallel = append(inParallel, c.Components[i])
		}
	}

	if len(inParallel) > 0 {
		sequence.Queue = append(sequence.Queue, inParallel)
	}

	return sequence
}

func contains(items []string, item string) bool {
	for i := range items {
		if item == items[i] {
			return true
		}
	}
	return false
}
