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
	CRDComponent             = "CRDs"
	CleanupComponent         = "cleaner"
	DeleteStrategyKey        = "delete_strategy"
	tblConfiguration  string = "inventory_cluster_configs"
	CleanerTypeKey           = "cleaner_type"
	CleanerCr                = "cleaner_CR"
	CleanerNamespace         = "cleaner_NS"
)

var (
	crdComponent              = &keb.Component{Component: CRDComponent, Namespace: "default"}
	cleanupComponent          = &keb.Component{Component: CleanupComponent, Namespace: "default"}
	cleanupNamespaceComponent = &keb.Component{
		Component: CleanupComponent,
		Namespace: "default",
		Configuration: []keb.Configuration{
			{Key: CleanerTypeKey, Value: CleanerNamespace},
		}}
	cleanupCRComponent = &keb.Component{
		Component: CleanupComponent,
		Namespace: "default",
		Configuration: []keb.Configuration{
			{Key: CleanerTypeKey, Value: CleanerCr},
		}}
)

type ClusterConfigurationEntity struct {
	Version        int64            `db:"readOnly"`
	RuntimeID      string           `db:"notNull"`
	ClusterVersion int64            `db:"notNull"` // Cluster entity primary key
	KymaVersion    string           `db:"notNull"`
	KymaProfile    string           `db:""`
	Components     []*keb.Component `db:"notNull,encrypt"`
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
	if component == CleanupComponent { //Cleanup is an artificial component which doesn't exist in the component list of any cluster
		return cleanupComponent
	}
	for _, comp := range c.Components {
		if comp.Component == component {
			return comp
		}
	}
	return nil
}

func (c *ClusterConfigurationEntity) GetReconciliationSequence(cfg *ReconciliationSequenceConfig) *ReconciliationSequence {
	reconSeq := newReconciliationSequence(cfg)
	reconSeq.addComponents(c.Components)
	return reconSeq
}

type ReconciliationSequence struct {
	Queue [][]*keb.Component
	cfg   *ReconciliationSequenceConfig
}

type ReconciliationSequenceConfig struct {
	PreComponents        [][]string
	DeleteStrategy       string
	ReconciliationStatus Status
}

func newReconciliationSequence(cfg *ReconciliationSequenceConfig) *ReconciliationSequence {
	reconSeq := &ReconciliationSequence{
		cfg: cfg,
	}

	reconSeq.Queue = append(reconSeq.Queue, []*keb.Component{ //CRDs are always processed at the very beginning (or at the very end in deletion)
		crdComponent,
	})

	// if a cluster is pending deletion, we need to add the cleanup component into the reconciliation
	if cfg.ReconciliationStatus.IsDeletionInProgress() {
		cleanupNamespaceComponent.Configuration = append(cleanupNamespaceComponent.Configuration, keb.Configuration{
			Key: DeleteStrategyKey, Value: cfg.DeleteStrategy,
		})
		// add cleanup of namespaces to the first index
		reconSeq.Queue = append([][]*keb.Component{
			{cleanupNamespaceComponent},
		}, reconSeq.Queue...)
	}

	return reconSeq
}

func (rs *ReconciliationSequence) addComponents(components []*keb.Component) {
	//for faster processing: map components by name
	compsByNameCache := func() map[string]*keb.Component {
		result := make(map[string]*keb.Component, len(components))
		for _, component := range components {
			result[component.Component] = component
		}
		return result
	}()

	//add pre-components to queue
	for _, preComponentGroup := range rs.cfg.PreComponents {
		var preComps []*keb.Component
		for _, preComponentName := range preComponentGroup {
			if preComp, ok := compsByNameCache[preComponentName]; ok {
				preComps = append(preComps, preComp)
				delete(compsByNameCache, preComp.Component) //remove pre-component from cache
				continue
			}
		}
		if len(preComps) > 0 {
			rs.Queue = append(rs.Queue, preComps)
		}
	}

	//add all remaining components in cache to queue
	var noPreComps []*keb.Component
	for _, comp := range compsByNameCache {
		noPreComps = append(noPreComps, comp)
	}
	if len(noPreComps) > 0 {
		rs.Queue = append(rs.Queue, noPreComps)
	}

	// add cleanup of CRs to the end, so that they are cleaned up first
	if rs.cfg.ReconciliationStatus.IsDeletionInProgress() {
		cleanupCRComponent.Configuration = append(cleanupCRComponent.Configuration, keb.Configuration{
			Key: DeleteStrategyKey, Value: rs.cfg.DeleteStrategy,
		})
		rs.Queue = append(rs.Queue, []*keb.Component{
			cleanupCRComponent,
		})
	}
}
