package configmaps

import (
	"context"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
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

func (cmr ConfigMapRepo) CreateServiceMappingConfig(name string) error {

	cm := &coreV1.ConfigMap{
		TypeMeta: v1.TypeMeta{Kind: "ConfigMap"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: cmr.Namespace,
		},
		Data: map[string]string{"servicemappings": "{}"},
	}

	return cmr.createConfigMap(cm)
}

func (cmr ConfigMapRepo) createConfigMap(cm *coreV1.ConfigMap) error {

	_, err := cmr.TargetClientSet.CoreV1().
		ConfigMaps(cmr.Namespace).
		Create(context.Background(), cm, v1.CreateOptions{})

	return err
}
