package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pvcsDeletionTimeout = 6 * time.Second
)

//deletePVCSAndWait marks PersistentVolumeClaims in a given namespace for deletion and then waits for a pre-defined time to ensure it's deleted.
//If a PVC can't be timely deleted it's probably because a StatefulSet (or a standard Pod) is still using it.
func (cmd *CliCleaner) deletePVCSAndWait(namespace string) error {

	retryFunc := func() error {
		err := cmd.deletePVCS(namespace)
		if err != nil {
			return err
		}
		err = cmd.waitUntilPVCSDeleted(namespace, pvcsDeletionTimeout)
		return err
	}

	var attempts uint = 3
	delay := time.Second * 2
	err := retry.Do(retryFunc, retry.Attempts(attempts), retry.Delay(delay))
	if err != nil {
		return err
	}

	return nil
}

func (cmd *CliCleaner) deletePVCS(namespace string) error {
	cmd.logger.Infof("Deleting remaining PersistentVolumeClaims in namespace \"%s\"", namespace)

	persistentVolumeClaims, err := cmd.findPVCS(namespace)
	if err != nil {
		return err
	}
	if persistentVolumeClaims == nil || len(persistentVolumeClaims.Items) == 0 {
		return nil
	}

	for i := range persistentVolumeClaims.Items {
		pvc := persistentVolumeClaims.Items[i]

		if err := cmd.k8s.Static().CoreV1().PersistentVolumeClaims(pvc.GetNamespace()).Delete(context.Background(), pvc.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}

		cmd.logger.Info(fmt.Sprintf("PersistentVolumeClaim \"%s/%s\" marked for deletion", namespace, pvc.GetName()))
	}

	return nil
}

func (cmd *CliCleaner) waitUntilPVCSDeleted(namespace string, waitTimeout time.Duration) error {
	cmd.logger.Infof("Waiting for deletion of PersistentVolumeClaims in namespace \"%s\"", namespace)

	timeout := time.After(waitTimeout)
	poll := time.NewTicker(waitTimeout/3 - 1)
	defer poll.Stop()
	for {
		select {
		case <-timeout:
			return errors.Errorf("timeout while waiting for deletion of PersistentVolumeClaims in namespace: %s", namespace)
		case <-poll.C:
			persistentVolumeClaims, err := cmd.findPVCS(namespace)
			if err != nil {
				return err
			}
			if persistentVolumeClaims == nil || len(persistentVolumeClaims.Items) == 0 {
				cmd.logger.Infof("All PersistentVolumeClaims in namespace \"%s\" deleted", namespace)
				return nil
			}
			cmd.logger.Infof("%d PersistentVolumeClaims in namespace \"%s\" are still present, waiting", len(persistentVolumeClaims.Items), namespace)
		}
	}
}

func (cmd *CliCleaner) findPVCS(namespace string) (*v1.PersistentVolumeClaimList, error) {

	nsFound, err := cmd.namespaceExists(namespace)
	if err != nil {
		return nil, err
	}
	if !nsFound {
		return nil, nil
	}

	res, err := cmd.k8s.Static().CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})

	if err != nil && apierr.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (cmd *CliCleaner) namespaceExists(namespace string) (bool, error) {
	_, err := cmd.k8s.Static().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})

	if err != nil && apierr.IsNotFound(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}
