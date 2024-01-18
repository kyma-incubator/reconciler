package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var kubeConfigCache = ttlcache.New[string, string](
	ttlcache.WithTTL[string, string](5*time.Minute),
	ttlcache.WithDisableTouchOnHit[string, string](),
)

// GetKubeConfigFromCache returns the kubeconfig from the cache if it is not expired.
// If it is expired, it will get the kubeconfig from the secret and set it in the cache.
func GetKubeConfigFromCache(logger *zap.SugaredLogger, clientSet *kubernetes.Clientset, runtimeID string) string {
	kubeConfigCache.DeleteExpired()
	kubeConfigFromCache := kubeConfigCache.Get(runtimeID)

	if kubeConfigFromCache.Value() == "" || kubeConfigFromCache.IsExpired() {
		kubeConfigCache.Delete(runtimeID)
		kubeConfig := getKubeConfigFromSecret(logger, clientSet, runtimeID)
		if kubeConfig != "" {
			SetKubeConfigInCache(runtimeID, kubeConfig)
		}
		return kubeConfig
	}

	return kubeConfigFromCache.Value()
}

// SetKubeConfigInCache sets the kubeconfig in the cache.
func SetKubeConfigInCache(key string, kubeconfig string) {
	kubeConfigCache.Set(key, kubeconfig, 5*time.Minute)
}

// getkubeConfigFromSecret gets the kubeconfig from the secret.
func getKubeConfigFromSecret(logger *zap.SugaredLogger, clientSet *kubernetes.Clientset, runtimeID string) string {
	secretResourceName := fmt.Sprintf("kubeconfig-%s", runtimeID)
	secret, err := getKubeConfigSecret(logger, clientSet, runtimeID, secretResourceName)
	if err != nil {
		return ""
	}

	kubeconfig, found := secret.Data["config"]
	if !found {
		logger.Errorf("Kubeconfig-secret '%s' for runtime '%s' does not include the data-key 'config'",
			secretResourceName, runtimeID)
	}

	return string(kubeconfig)
}

// getKubeConfigSecret gets the kubeconfig secret from the cluster.
func getKubeConfigSecret(logger *zap.SugaredLogger, clientSet *kubernetes.Clientset,
	runtimeID, secretResourceName string) (secret *corev1.Secret, err error) {

	secret, err = clientSet.CoreV1().Secrets("kcp-system").Get(context.TODO(), secretResourceName, metav1.GetOptions{})
	if err != nil {
		if k8serr.IsNotFound(err) { // accepted failure
			logger.Debugf("Cluster inventory cannot find a kubeconfig-secret '%s' for cluster with runtimeID %s: %w",
				secretResourceName, runtimeID, err)
			return nil, err
		} else if k8serr.IsForbidden(err) { // configuration failure
			logger.Warnf("Cluster inventory is not allowed to lookup kubeconfig-secret '%s' for cluster with runtimeID %s: %w",
				secretResourceName, runtimeID, err)
			return nil, err
		}
		logger.Errorf("Cluster inventory failed to lookup kubeconfig-secret '%s' for cluster with runtimeID %s: %s",
			secretResourceName, runtimeID, err)
		return nil, err

	}
	logger.Infof("Successfully retrieved kubeconfig-secret '%s' for cluster with runtimeID %s",
		secretResourceName, runtimeID)
	return secret, nil
}
