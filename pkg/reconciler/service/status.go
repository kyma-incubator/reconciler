package service

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	k8s "github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createOrUpdateStatusCm(ctx context.Context, componentName string, status reconciler.Status, version string, kubeclient k8s.Client) error {
	configMapName := fmt.Sprintf("%s-status", componentName)
	namespace := "kyma-system"
	clientset, err := kubeclient.Clientset()
	if err != nil {
		return err
	}

	// Get ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// ConfigMap does not exist, create new one
		configFile := map[string]string{
			"name":                componentName,
			"version":             version,
			"status":              string(status),
			"last-reconciliation": "",
		}
		cm := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
				Labels:    map[string]string{}, // TODO fill in correct labels
			},
			Data: configFile,
		}
		_, err := clientset.CoreV1().ConfigMaps("my-namespace").Create(ctx, &cm, metav1.CreateOptions{})
		if err != nil {
			return err
		}

	} else if err != nil {
		// Error while fetching ConfigMap
		return err // TODO better logging
	} else {
		// Update existing ConfigMap
		configMap.Data["version"] = version
		configMap.Data["status"] = string(status)
		if status == reconciler.StatusSuccess {
			configMap.Data["last-reconciliation"] = time.Now().String()
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
