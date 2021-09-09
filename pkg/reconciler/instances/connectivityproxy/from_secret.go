package connectivityproxy

import (
	"context"
	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

type FromSecret struct {
	Namespace string
	Name      string

	inCluster k8s.Interface
}

func (fs *FromSecret) Get() (*coreV1.Secret, error) {
	return fs.inCluster.CoreV1().Secrets(fs.Namespace).
		Get(context.Background(), fs.Name, v1.GetOptions{})
}
