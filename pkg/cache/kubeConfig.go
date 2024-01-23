package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	ttl             = getTTL()
	kubeConfigCache = ttlcache.New[string, string](
		ttlcache.WithTTL[string, string](ttl),
		ttlcache.WithDisableTouchOnHit[string, string](),
	)
)

// GetKubeConfigFromCache returns the kubeconfig from the cache if it is not expired.
// If it is expired, it will get the kubeconfig from the secret and set it in the cache.
func GetKubeConfigFromCache(logger *zap.SugaredLogger, clientSet *kubernetes.Clientset, runtimeID string) (string,
	error) {
	kubeConfigCache.DeleteExpired()

	if kubeConfigCache.Has(runtimeID) {
		logger.Infof("Kubeconfig cache found kubeconfig for cluster (runtimeID: %s) in cache", runtimeID)
		cacheEntry := kubeConfigCache.Get(runtimeID)
		if cacheEntry.Value() == "" {
			return "", fmt.Errorf("Kubeconfig cache failed to find valid kubeconfig found for cluster (runtimeID: %s), will retry the kubeconfig retrieval after %s",
				runtimeID, cacheEntry.ExpiresAt())
		}
		return cacheEntry.Value(), nil
	}

	kubeConfig, err := getKubeConfigFromSecret(logger, clientSet, runtimeID)
	if err == nil {
		logger.Infof("Kubeconfig cache retrieved kubeconfig for cluster (runtimeID: %s) from secret: caching it now",
			runtimeID)
		kubeConfigCache.Set(runtimeID, kubeConfig, ttl)
	} else {
		// HACK: workaround to avoid that too many non-existing clusters lead to peformance issues
		logger.Infof("Kubeconfig cache failed to get kubeconfig for cluster (runtimeID: %s) from secret - will cache empty string: %s",
			runtimeID, err)
		kubeConfigCache.Set(runtimeID, "", ttl)
	}

	return kubeConfig, err
}

// getkubeConfigFromSecret gets the kubeconfig from the secret.
func getKubeConfigFromSecret(logger *zap.SugaredLogger, clientSet *kubernetes.Clientset, runtimeID string) (string,
	error) {
	secretResourceName := fmt.Sprintf("kubeconfig-%s", runtimeID)
	secret, err := getKubeConfigSecret(logger, clientSet, runtimeID, secretResourceName)
	if err != nil {
		return "", err
	}

	kubeconfig, found := secret.Data["config"]
	if !found {
		return "", fmt.Errorf("Kubeconfig cache found kubeconfig-secret '%s' for runtime '%s' which does not include the data-key 'config'",
			secretResourceName, runtimeID)
	}

	if len(kubeconfig) == 0 {
		return "", fmt.Errorf("Kubeconfig cache found kubeconfig-secret '%s' for runtime '%s' which includes an empty kubeconfig string",
			secretResourceName, runtimeID)
	}

	return string(kubeconfig), nil
}

// getKubeConfigSecret gets the kubeconfig secret from the cluster.
func getKubeConfigSecret(logger *zap.SugaredLogger, clientSet *kubernetes.Clientset,
	runtimeID, secretResourceName string) (secret *corev1.Secret, err error) {

	secret, err = clientSet.CoreV1().Secrets("kcp-system").Get(context.TODO(), secretResourceName, metav1.GetOptions{})
	if err != nil {
		if k8serr.IsNotFound(err) { // accepted failure
			logger.Infof("Kubeconfig cache cannot find a kubeconfig-secret '%s' for cluster with runtimeID %s: %s",
				secretResourceName, runtimeID, err)
			return nil, err
		} else if k8serr.IsForbidden(err) { // configuration failure
			logger.Errorf("Kubeconfig cache is not allowed to lookup kubeconfig-secret '%s' for cluster with runtimeID %s: %s",
				secretResourceName, runtimeID, err)
			return nil, err
		}
		logger.Errorf("Kubeconfig cache failed to lookup kubeconfig-secret '%s' for cluster with runtimeID %s: %s",
			secretResourceName, runtimeID, err)
		return nil, err

	}
	return secret, nil
}

func getTTL() time.Duration {
	ttl := os.Getenv("KUBECONFIG_CACHE_TTL")
	if ttl == "" {
		return 30 * time.Minute
	}
	ttlDuration, err := time.ParseDuration(ttl)
	if err != nil {
		return 30 * time.Minute
	}
	return ttlDuration
}
