package deprecation

import (
	"context"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const Namespace = "hydra-deprecated"

func NamespaceExists(ctx context.Context, client kubernetes.Interface) (bool, error) {
	_, err := client.CoreV1().Namespaces().Get(ctx, Namespace, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "Could not get %s namespace", Namespace)
	}
	return true, nil
}
