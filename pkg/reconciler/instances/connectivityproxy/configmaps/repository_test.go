package configmaps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestCreatingServiceMappingsConfigMap(t *testing.T) {

	t.Run("Should create service mapping configmap", func(t *testing.T) {

		fakeClientSet := fake.NewSimpleClientset()
		repo := NewConfigMapRepo("test-namespace", fakeClientSet)

		ctx := context.Background()

		err := repo.CreateServiceMappingConfig(ctx, "test")
		require.NoError(t, err)

		_, err = fakeClientSet.CoreV1().ConfigMaps("test-namespace").Get(context.Background(), "test", metav1.GetOptions{})
		require.NoError(t, err)
	})

	t.Run("Should not replace already existing configmap", func(t *testing.T) {
		// given
		fakeClientSet := fake.NewSimpleClientset()

		namespace := "test-namespace"
		secretName := "secret-name"
		expectedData := map[string]string{
			"test": "me",
		}

		existingConfigmap := &coreV1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: expectedData,
		}

		_, err := fakeClientSet.CoreV1().
			ConfigMaps(namespace).
			Create(context.Background(), existingConfigmap, metav1.CreateOptions{})

		require.NoError(t, err)

		repo := NewConfigMapRepo(namespace, fakeClientSet)

		ctx := context.Background()

		// when
		err = repo.CreateServiceMappingConfig(ctx, secretName)
		require.NoError(t, err)

		// then
		actual, err := fakeClientSet.CoreV1().ConfigMaps(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, expectedData, actual.Data)
	})
}

func TestFixingConfiguration(t *testing.T) {
	t.Run("Should not attempt to fix configuration if configmap doesn't exist", func(t *testing.T) {
		fakeClientSet := fake.NewSimpleClientset()
		repo := NewConfigMapRepo("test-namespace", fakeClientSet)

		// when
		err := repo.FixConfiguration("test-namespace", "connectivity-proxy", "cp.example.com\"")

		// then
		require.NoError(t, err)
	})

	t.Run("Should return error when config map lacks expected key", func(t *testing.T) {
		// given
		fakeClientSet := fake.NewSimpleClientset()

		configMap := &coreV1.ConfigMap{
			TypeMeta: metav1.TypeMeta{Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "connectivity-proxy",
				Namespace: "test-namespace",
			},
			Data: nil,
		}
		_, err := fakeClientSet.CoreV1().ConfigMaps("test-namespace").Create(context.Background(), configMap, metav1.CreateOptions{})
		require.NoError(t, err)

		repo := NewConfigMapRepo("test-namespace", fakeClientSet)

		// when
		err = repo.FixConfiguration("test-namespace", "connectivity-proxy", "cp.example.com\"")

		// then
		require.Error(t, err)
	})

	cmToFix := `
apiVersion: v1
kind: ConfigMap
metadata:
  annotations:
    reconciler.kyma-project.io/skip-rendering-on-upgrade: "true"
  labels:
    connectivityproxy.sap.com/restart: connectivity-proxy.kyma-system
  name: connectivity-proxy
  namespace: test-namespace
data:
  connectivity-proxy-config.yml: | 
    highAvailabilityMode: "off"
    integration:
      auditlog:
        mode: console
      connectivityService:
        serviceCredentialsKey: service_key
    servers:
      businessDataTunnel:
        externalHost: cc-proxy.api.example.com
        externalPort: 443
      proxy:
        http:
          enableProxyAuthorization: false
          enabled: true
        rfcAndLdap:
          enableProxyAuthorization: false
          enabled: true
        socks5:
          enableProxyAuthorization: false
          enabled: true
    subaccountId: subaccountId
    subaccountSubdomain: subaccountSubdomain
    tenantMode: dedicated`

	createConfigMapFunc := func(fakeClientSet *fake.Clientset) {
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(cmToFix), nil, nil)
		require.NoError(t, err)

		configMap := obj.(*coreV1.ConfigMap)

		_, err = fakeClientSet.CoreV1().ConfigMaps("test-namespace").Create(context.Background(), configMap, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	getExternalHostFromConfigMap := func(configMap *coreV1.ConfigMap) string {
		configYaml, ok := configMap.Data["connectivity-proxy-config.yml"]
		require.True(t, ok)

		s := &struct {
			Servers *struct {
				BusinessDataTunnel *struct {
					ExternalHost string `yaml:"externalHost"`
				} `yaml:"businessDataTunnel"`
			} `yaml:"servers"`
		}{}

		err := yaml.Unmarshal([]byte(configYaml), s)
		require.NoError(t, err)

		return s.Servers.BusinessDataTunnel.ExternalHost
	}

	t.Run("Should attempt to fix configuration configmap", func(t *testing.T) {
		// given
		expectedHost := "cp.example.com"
		fakeClientSet := fake.NewSimpleClientset()
		repo := NewConfigMapRepo("test-namespace", fakeClientSet)

		createConfigMapFunc(fakeClientSet)

		// when
		err := repo.FixConfiguration("test-namespace", "connectivity-proxy", "cp.example.com")

		// then
		require.NoError(t, err)

		updatedConfigMap, err := fakeClientSet.CoreV1().ConfigMaps("test-namespace").Get(context.Background(), "connectivity-proxy", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "connectivity-proxy.kyma-system", updatedConfigMap.GetLabels()["connectivityproxy.sap.com/restart"])

		actualExternalHost := getExternalHostFromConfigMap(updatedConfigMap)
		require.Equal(t, expectedHost, actualExternalHost)

		// when
		err = repo.FixConfiguration("test-namespace", "connectivity-proxy", "cp.example.com")

		// then
		require.NoError(t, err)

		updatedConfigMap, err = fakeClientSet.CoreV1().ConfigMaps("test-namespace").Get(context.Background(), "connectivity-proxy", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "connectivity-proxy.kyma-system", updatedConfigMap.GetLabels()["connectivityproxy.sap.com/restart"])

		actualExternalHost = getExternalHostFromConfigMap(updatedConfigMap)
		require.Equal(t, expectedHost, actualExternalHost)
	})

	type testIncorrectYamlStructure struct {
		configYaml  string
		description string
	}

	missingServerKeyYaml := `highAvailabilityMode: "off"`
	missingTunnelKey := `
servers:
  businessDataTunnelMissing:
    externalHost: cc-proxy.api.example.com
    externalPort: 443
`
	missingExternalHostKey := `
    servers:
      businessDataTunnel:
        externalHostMissing: cc-proxy.api.example.com 
`
	tests := []testIncorrectYamlStructure{
		{description: "should return error when config map lacks servers key", configYaml: missingServerKeyYaml},
		{description: "should return error when config map lacks businessDataTunnel key", configYaml: missingTunnelKey},
		{description: "should return error when config map lacks externalHost key", configYaml: missingExternalHostKey},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeClientSet := fake.NewSimpleClientset()
			configMap := &coreV1.ConfigMap{
				TypeMeta: metav1.TypeMeta{Kind: "ConfigMap"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "connectivity-proxy",
					Namespace: "test-namespace",
				},
				Data: map[string]string{"connectivity-proxy-config.yml": tc.configYaml},
			}

			_, err := fakeClientSet.CoreV1().ConfigMaps("test-namespace").Create(context.Background(), configMap, metav1.CreateOptions{})
			require.NoError(t, err)

			repo := NewConfigMapRepo("test-namespace", fakeClientSet)

			// when
			err = repo.FixConfiguration("test-namespace", "connectivity-proxy", "cp.api.example.com")

			// then
			require.Error(t, err)
		})
	}

}
