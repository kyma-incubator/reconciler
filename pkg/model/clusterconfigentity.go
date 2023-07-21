package model

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"go.uber.org/zap"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/keb"
)

const (
	CRDComponent             = "CRDs"
	CleanupComponent         = "cleaner"
	DeleteStrategyKey        = "delete_strategy"
	tblConfiguration  string = "inventory_cluster_configs"
)

var (
	crdComponent     = &keb.Component{Component: CRDComponent, Namespace: "default"}
	cleanupComponent = &keb.Component{Component: CleanupComponent, Namespace: "default"}
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
	reconSeq.addComponents(c.nonMigratedComponents(cfg))
	return reconSeq
}

func (c *ClusterConfigurationEntity) nonMigratedComponents(cfg *ReconciliationSequenceConfig) []*keb.Component {
	logger := log.NewLogger(false)

	if !isKubeconfig(cfg.Kubeconfig) {
		logger.Warnf("Kubeconfig is missing or invalid for cluster '%s': not able to verify which components were "+
			"already migrated. We assume this is a test case and consider all components for this reconciliation.",
			c.RuntimeID)
		return c.Components
	}

	restConfig, err := clientcmd.NewClientConfigFromBytes([]byte(cfg.Kubeconfig))
	if err != nil {
		return c.stopAndLogK8sError(logger, "restConfig", err)
	}

	clientConfig, err := restConfig.ClientConfig()
	if err != nil {
		return c.stopAndLogK8sError(logger, "clientConfig", err)
	}

	kubeClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return c.stopAndLogK8sError(logger, "kubeClient", err)
	}

	//check which of the configured CRDs exist on the cluster
	var migratedComponents = make(map[string]bool, len(cfg.ComponentCRDs))
	for compName, compGVK := range cfg.ComponentCRDs {
		gvr := schema.GroupVersionResource{
			Group:    compGVK.Group,
			Version:  compGVK.Version,
			Resource: compGVK.Kind,
		}
		_, err := kubeClient.Resource(gvr).List(context.Background(), v1.ListOptions{})
		if err == nil {
			logger.Infof("Found CRD '%s' on cluster '%s' which indicates that component '%s' "+
				"is migrated to new reconicler system: skipping it from reconciliation",
				gvr, c.RuntimeID, compName)
			migratedComponents[strings.ToLower(compName)] = true
		} else if !k8serr.IsNotFound(err) {
			logger.Errorf("Failed to retrieve CRD '%s:%s' from cluster '%s': %s",
				compGVK.Group, compGVK.Version, c.RuntimeID, err)
			logger.Warnf("It is assumed that component '%s' on cluster '%s' "+
				"is already migrated and the old reconicler will NOT reconcile it now", compName, c.RuntimeID)
			migratedComponents[strings.ToLower(compName)] = true
		}
	}

	var result []*keb.Component
	for _, comp := range c.Components {
		//ignore all migrated components
		if migratedComponents[strings.ToLower(comp.Component)] {
			continue
		}
		//ignore component if the SKIP_COMPONENT_XYZ env-var is defined
		envVar := fmt.Sprintf("SKIP_COMPONENT_%s", strings.ReplaceAll(strings.ToUpper(comp.Component), "-", "_"))
		skipComp := os.Getenv(envVar)
		if strings.ToLower(skipComp) == "true" || skipComp == "1" {
			logger.Info("Skipping component %s (env-var: %s = $s)", comp.Component, envVar, skipComp)
			continue
		}
		result = append(result, comp)
	}

	return result
}

func isKubeconfig(kubeconfig string) bool {
	byteKubecfg := []byte(kubeconfig)
	cfg, err := clientcmd.Load(byteKubecfg)
	return err == nil && !api.IsConfigEmpty(cfg) && len(cfg.Clusters) > 0 && len(cfg.AuthInfos) > 0
}

func (c *ClusterConfigurationEntity) stopAndLogK8sError(logger *zap.SugaredLogger, subject string, err error) []*keb.Component {
	logger.Errorf("Failed to create %s and cannot verify which of the components of cluster '%s' "+
		"are already managed by the operator-based reconciler: %s", subject, c.RuntimeID, err)
	logger.Warnf("Caused by missig K8s client, old reconciler will not reconcile "+
		"any component on cluster '%s'", c.RuntimeID)
	return make([]*keb.Component, 0)
}

type ReconciliationSequence struct {
	Queue         [][]*keb.Component
	preComponents [][]string
}

type ReconciliationSequenceConfig struct {
	PreComponents        [][]string
	DeleteStrategy       string
	ComponentCRDs        map[string]config.ComponentCRD
	ReconciliationStatus Status
	Kubeconfig           string
}

func newReconciliationSequence(cfg *ReconciliationSequenceConfig) *ReconciliationSequence {
	reconSeq := &ReconciliationSequence{
		preComponents: cfg.PreComponents,
	}
	reconSeq.Queue = append(reconSeq.Queue, []*keb.Component{ //CRDs are always processed at the very beginning (or at the very end in deletion)
		crdComponent,
	})

	// if a cluster is pending deletion, we need to add the cleanup component into the reconciliation
	if cfg.ReconciliationStatus.IsDeletionInProgress() {
		cleanupComponent.Configuration = append(cleanupComponent.Configuration, keb.Configuration{
			Key: DeleteStrategyKey, Value: cfg.DeleteStrategy,
		})
		reconSeq.Queue = append(reconSeq.Queue, []*keb.Component{
			cleanupComponent,
		})
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
	for _, preComponentGroup := range rs.preComponents {
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
}
