package cleanup

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cmd *CliCleaner) deleteKymaNamespaces() error {
	if !cmd.dropKymaNamespaces {
		return nil
	}
	cmd.logger.Info("Deleting Kyma Namespaces")

	var wg sync.WaitGroup
	wg.Add(len(cmd.namespaces))
	finishedCh := make(chan bool)
	errorCh := make(chan error)

	for _, namespace := range cmd.namespaces {
		go func(namespaceName string) {
			defer wg.Done()
			err := retry.Do(func() error {
				cmd.logger.Info(fmt.Sprintf("Deleting Namespace \"%s\"", namespaceName))

				// check if NS exists
				ns, err := cmd.k8s.Static().CoreV1().Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})
				if err != nil && !apierr.IsNotFound(err) {
					errorCh <- err
				} else if apierr.IsNotFound(err) {
					return nil
				}

				// delete NS
				if err := cmd.k8s.Static().CoreV1().Namespaces().Delete(context.Background(), namespaceName, metav1.DeleteOptions{}); err != nil && !apierr.IsNotFound(err) {
					errorCh <- err
				}

				return errors.Wrapf(err, "\"%s\" Namespace still exists in \"%s\" Phase", ns.Name, ns.Status.Phase)
			})
			if err != nil {
				errorCh <- err
				return
			}

			cmd.logger.Info(fmt.Sprintf("\"%s\" Namespace is removed", namespaceName))
		}(namespace)
	}

	go func() {
		wg.Wait()
		close(errorCh)
		close(finishedCh)
	}()

	// process deletion results
	var errWrapped error
	for {
		select {
		case <-finishedCh:
			if errWrapped == nil {
				cmd.logger.Info("All Kyma Namespaces marked for deletion")
			}
			return errWrapped
		case err := <-errorCh:
			if err != nil {
				if errWrapped == nil {
					errWrapped = err
				} else {
					errWrapped = errors.Wrap(err, errWrapped.Error())
				}
			}
		}
	}
}

func (cmd *CliCleaner) waitForNamespaces() error {

	cmd.logger.Info("Waiting for Namespace deletion")

	timeout := time.After(cmd.namespaceTimeout)
	poll := time.NewTicker(6 * time.Second)
	defer poll.Stop()
	for {
		select {
		case <-timeout:
			cmd.logger.Info("Timeout while removing namespaced, trying to remove finalizers..")
			return cmd.removeAllNsFinalizers()
		case <-poll.C:
			if err := cmd.removeResourcesFinalizers(); err != nil {
				return err
			}
			ok, err := cmd.checkKymaNamespaces()
			if err != nil {
				return err
			} else if ok {
				return nil
			}
		}
	}
}

func (cmd *CliCleaner) checkKymaNamespaces() (bool, error) {
	namespaceList, err := cmd.k8s.Static().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	if namespaceList.Size() == 0 {
		cmd.logger.Info("No remaining Kyma Namespaces found")
		return true, nil
	}

	for i := range namespaceList.Items {
		if contains(cmd.namespaces, namespaceList.Items[i].Name) {
			cmd.logger.Info(fmt.Sprintf("Namespace %s still in state '%s'", namespaceList.Items[i].Name, namespaceList.Items[i].Status.Phase))
			return false, nil
		}
	}

	cmd.logger.Info("No remaining Kyma Namespaces found")

	return true, nil
}

func (cmd *CliCleaner) removeAllNsFinalizers() error {
	namespaceList, err := cmd.k8s.Static().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, namespace := range namespaceList.Items {
		if err = cmd.removeKymaNsFinalizers(namespace); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *CliCleaner) removeKymaNsFinalizers(namespace v1.Namespace) error {
	// set terminating status
	updatedNamespace := &v1.Namespace{
		ObjectMeta: namespace.ObjectMeta,
		Spec:       namespace.Spec,
		Status: v1.NamespaceStatus{
			Phase: v1.NamespaceTerminating,
		},
	}
	if namespace.Status.Phase != v1.NamespaceTerminating {
		_, err := cmd.k8s.Static().CoreV1().Namespaces().Update(context.TODO(), updatedNamespace, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update namespace %s: %w", updatedNamespace.Name, err)
		}
	}

	// Remove FinalizerKubernetes
	if len(updatedNamespace.Spec.Finalizers) != 0 {
		finalizerSet := sets.NewString()
		for i := range namespace.Spec.Finalizers {
			if namespace.Spec.Finalizers[i] != v1.FinalizerKubernetes {
				finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
			}
		}
		updatedNamespace.Spec.Finalizers = make([]v1.FinalizerName, 0, len(finalizerSet))
		for _, value := range finalizerSet.List() {
			updatedNamespace.Spec.Finalizers = append(updatedNamespace.Spec.Finalizers, v1.FinalizerName(value))
		}
		_, err := cmd.k8s.Static().CoreV1().Namespaces().Finalize(context.TODO(), updatedNamespace, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update namespace %s: %w", updatedNamespace.Name, err)
		}
	}

	err := cmd.k8s.Static().CoreV1().Namespaces().Delete(context.TODO(), updatedNamespace.Name, metav1.DeleteOptions{})
	if client2.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", updatedNamespace.Name, err)
	}
	cmd.logger.Infof("namespace deleted %s", updatedNamespace.Name)
	return nil
}
