package ingressgateway

import (
	"context"

	"github.com/go-yaml/yaml"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NumTrustedProxies *int

const (
	CMNamespace string = "istio-system"
	CMName      string = "istio"
)

// GetNumTrustedProxyFromIstioCM fetches current cluster configuration for "numTrustedProxies" from istio-system/istio ConfigMap.
// Returns nil if no configuration is present (default configuration).
func GetNumTrustedProxyFromIstioCM(ctx context.Context, client client.Client) (NumTrustedProxies, error) {
	cm := corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{Namespace: CMNamespace, Name: CMName}, &cm)

	if k8serrors.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	val, ok := cm.Data["mesh"]
	if !ok {
		return nil, nil
	}

	unmarshalledConfig := struct {
		DefaultConfig struct {
			GatewayTopology struct {
				NumTrustedProxies *int `yaml:"numTrustedProxies,omitempty"`
			} `yaml:"gatewayTopology"`
		} `yaml:"defaultConfig"`
	}{}

	err = yaml.Unmarshal([]byte(val), &unmarshalledConfig)
	if err != nil {
		return nil, err
	}

	return unmarshalledConfig.DefaultConfig.GatewayTopology.NumTrustedProxies, nil
}
