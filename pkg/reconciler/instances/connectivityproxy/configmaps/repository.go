package configmaps

import (
	"context"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	coreV1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
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

// FixConfiguration function corrects the exposed channel's url. Previous versions used cc-proxy.api prefix, and now we use cp prefix.
// This is 2.9.2 specific.
func (cmr ConfigMapRepo) FixConfiguration(namespace, name, tunnelURL string) error {
	configMap, err := cmr.TargetClientSet.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, v1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// ConfigMap is not created yet, nothing to do
		return nil
	}

	if err != nil {
		return errors.Wrap(err, "cannot get config map")
	}

	err = fixYamlConfig(configMap.Data, tunnelURL)
	if err != nil {
		return err
	}

	labels := configMap.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[restartLabel] = connectivityProxyService

	_, err = cmr.TargetClientSet.CoreV1().ConfigMaps(namespace).Update(context.Background(), configMap, v1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func fixYamlConfig(configMapData map[string]string, tunnelURL string) error {
	config, ok := configMapData[configurationKey]
	if !ok {
		return errors.New("cannot find configuration key in the config map")
	}

	configYaml, err := readYaml(config)
	if err != nil {
		return err
	}

	currentTunnelURL, err := getTunnelURL(configYaml)
	if err != nil {
		return err
	}

	if currentTunnelURL == tunnelURL {
		return nil
	}

	err = replaceTunnelURL(configYaml, tunnelURL)
	if err != nil {
		return err
	}

	modifiedYaml, err := writeYaml(configYaml)
	if err != nil {
		return err
	}

	configMapData[configurationKey] = modifiedYaml

	return nil
}

func readYaml(config string) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(config), &data)
	if err != nil {
		return map[string]interface{}{}, err
	}

	return data, err
}

func writeYaml(configMap map[string]interface{}) (string, error) {
	yamlBytes, err := yaml.Marshal(&configMap)
	if err != nil {
		return "", err
	}
	return string(yamlBytes), nil
}

func getTunnelURL(config map[string]interface{}) (string, error) {
	businessDataTunnelMap, err := getTunnelConfig(config)
	if err != nil {
		return "", err
	}

	externalHost, ok := businessDataTunnelMap["externalHost"].(string)
	if !ok {
		return "", errors.New("servers key type must be string")
	}

	return externalHost, nil
}

func replaceTunnelURL(config map[string]interface{}, tunnelURL string) error {
	businessDataTunnelMap, err := getTunnelConfig(config)
	if err != nil {
		return err
	}

	businessDataTunnelMap["externalHost"] = tunnelURL

	return nil
}

func getTunnelConfig(config map[string]interface{}) (map[string]interface{}, error) {
	servers, ok := config["servers"]
	if !ok {
		return map[string]interface{}{}, errors.New("failed to find servers key in config")
	}

	serversMap, ok := servers.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, errors.New("servers key type must be map[string]interface{}")
	}

	businessDataTunnel, ok := serversMap["businessDataTunnel"]
	if !ok {
		return map[string]interface{}{}, errors.New("failed to find businessDataTunnel key in config")
	}

	businessDataTunnelMap, ok := businessDataTunnel.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, errors.New("businessDataTunnel key type must be map[string]interface{}")
	}

	return businessDataTunnelMap, nil
}
