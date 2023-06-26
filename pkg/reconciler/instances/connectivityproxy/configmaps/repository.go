package configmaps

import (
	"context"
	"github.com/pkg/errors"
	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"strings"
)

const (
	configurationKey         = "connectivity-proxy-config.yml"
	restartLabel             = "connectivityproxy.sap.com/restart"
	connectivityProxyService = "connectivity-proxy.kyma-system"
)

type ConfigMapRepo struct {
	Namespace       string
	TargetClientSet k8s.Interface
}

func NewConfigMapRepo(namespace string, targetClientSet k8s.Interface) *ConfigMapRepo {

	if namespace == "" {
		namespace = "default"
	}

	return &ConfigMapRepo{
		Namespace:       namespace,
		TargetClientSet: targetClientSet,
	}
}

func (cmr ConfigMapRepo) CreateServiceMappingConfig(ctx context.Context, name string) error {

	cm := &coreV1.ConfigMap{
		TypeMeta: v1.TypeMeta{Kind: "ConfigMap"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: cmr.Namespace,
		},
		Data: map[string]string{"servicemappings": "{}"},
	}

	return cmr.createConfigMap(ctx, cm)
}

func (cmr ConfigMapRepo) createConfigMap(ctx context.Context, cm *coreV1.ConfigMap) error {
	name := cm.GetName()

	_, err := cmr.TargetClientSet.
		CoreV1().
		ConfigMaps(cmr.Namespace).
		Get(ctx, name, v1.GetOptions{})

	if err == nil {
		return nil
	}

	if !k8serrors.IsNotFound(err) {
		return err
	}

	_, err = cmr.TargetClientSet.CoreV1().
		ConfigMaps(cmr.Namespace).
		Create(context.Background(), cm, v1.CreateOptions{})

	return err
}

// FixConfiguration function corrects the exposed channel's url. Previous versions used cc-proxy prefix, and now we use cp prefix.
// This is 2.9.2 specific.
func (cmr ConfigMapRepo) FixConfiguration(namespace, name string) error {
	configMap, err := cmr.TargetClientSet.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, v1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// ConfigMap is not created yet, nothing to do
		return nil
	}

	if err != nil {
		return errors.Wrap(err, "cannot get config map")
	}

	config, ok := configMap.Data[configurationKey]
	if !ok {
		return errors.New("cannot find configuration key in the config map")
	}

	newConfig := strings.Replace(config, "externalHost: cc-proxy.", "externalHost: cp.", -1)
	if newConfig == config {
		// Already replaced, nothing to do
		return nil
	}

	labels := configMap.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[restartLabel] = connectivityProxyService

	configMap.Data[configurationKey] = newConfig
	_, err = cmr.TargetClientSet.CoreV1().ConfigMaps(namespace).Update(context.Background(), configMap, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
