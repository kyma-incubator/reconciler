package model

import (
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"reflect"
	"time"
)

const viewActiveInventoryStatusDetails string = "v_active_inventory_cluster_latest_status_details"

type ActiveInventoryClusterStatusDetailsEntity struct {
	StatusID        int64     `db:"readOnly"`
	StatusCreatedAt time.Time `db:"readOnly"`

	ConfigID        int64     `db:"readOnly"`
	ConfigCreatedAt time.Time `db:"readOnly"`

	ClusterID        int64     `db:"readOnly"`
	ClusterCreatedAt time.Time `db:"readOnly"`

	RuntimeID string            `db:"readOnly"`
	Runtime   *keb.RuntimeInput `db:"readOnly"`

	Metadata       *keb.Metadata    `db:"readOnly"`
	Kubeconfig     string           `db:"notNull,encrypt"`
	Status         Status           `db:"readOnly"`
	KymaVersion    string           `db:"readOnly"`
	KymaProfile    string           `db:"readOnly"`
	Contract       int64            `db:"readOnly"`
	Components     []*keb.Component `db:"readOnly,encrypt"`
	Administrators []string         `db:"readOnly"`
}

func (a *ActiveInventoryClusterStatusDetailsEntity) String() string {
	return fmt.Sprintf("ActiveInventoryClusterStatusDetailsEntity [RuntimeID=%v,ConfigID=%v,ClusterID=%v]", a.RuntimeID, a.ConfigID, a.ClusterID)
}

func (a *ActiveInventoryClusterStatusDetailsEntity) New() db.DatabaseEntity {
	return &ActiveInventoryClusterStatusDetailsEntity{}
}

func (a *ActiveInventoryClusterStatusDetailsEntity) Marshaller() *db.EntityMarshaller {
	marshaller := db.NewEntityMarshaller(&a)
	marshaller.AddUnmarshaller("StatusCreatedAt", convertTimestampToTime)
	marshaller.AddUnmarshaller("ConfigCreatedAt", convertTimestampToTime)
	marshaller.AddUnmarshaller("ClusterCreatedAt", convertTimestampToTime)
	marshaller.AddUnmarshaller("Status", func(value interface{}) (interface{}, error) {
		clusterStatus, err := NewClusterStatus(Status(value.(string)))
		if err == nil {
			return clusterStatus.Status, nil
		}
		return "", err
	})
	marshaller.AddUnmarshaller("Runtime", func(value interface{}) (interface{}, error) {
		var runtimeInput *keb.RuntimeInput
		err := json.Unmarshal([]byte(value.(string)), &runtimeInput)
		return runtimeInput, err
	})
	marshaller.AddUnmarshaller("Metadata", func(value interface{}) (interface{}, error) {
		var metadata *keb.Metadata
		err := json.Unmarshal([]byte(value.(string)), &metadata)
		return metadata, err
	})
	marshaller.AddUnmarshaller("Administrators", func(value interface{}) (interface{}, error) {
		var result []string
		err := json.Unmarshal([]byte(value.(string)), &result)
		return result, err
	})

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

	marshaller.AddMarshaller("Administrators", convertInterfaceToJSONString)
	marshaller.AddMarshaller("Components", convertInterfaceToJSONString)
	marshaller.AddMarshaller("Runtime", convertInterfaceToJSONString)
	marshaller.AddMarshaller("Metadata", convertInterfaceToJSONString)

	return marshaller
}

func (a *ActiveInventoryClusterStatusDetailsEntity) Table() string {
	return viewActiveInventoryStatusDetails
}

func (a *ActiveInventoryClusterStatusDetailsEntity) Equal(other db.DatabaseEntity) bool {
	if other == nil {
		return false
	}
	otherA, ok := other.(*ActiveInventoryClusterStatusDetailsEntity)

	if ok {
		ok = a.ConfigID == otherA.ConfigID && a.Status == otherA.Status &&
			a.RuntimeID == otherA.RuntimeID &&
			a.ClusterID == otherA.ClusterID &&
			a.KymaVersion == otherA.KymaVersion &&
			a.KymaProfile == otherA.KymaProfile &&
			a.Contract == otherA.Contract &&
			a.ClusterCreatedAt == otherA.ClusterCreatedAt &&
			a.ConfigCreatedAt == otherA.ConfigCreatedAt &&
			a.StatusCreatedAt == otherA.StatusCreatedAt &&
			reflect.DeepEqual(a.Components, otherA.Components) &&
			reflect.DeepEqual(a.Administrators, otherA.Administrators) &&
			reflect.DeepEqual(a.Runtime, otherA.Runtime) &&
			reflect.DeepEqual(a.Metadata, otherA.Metadata)
	}

	return ok
}
