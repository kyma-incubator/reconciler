package gardener

import (
	"testing"

	gardenerTypes "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_annotate(t *testing.T) {
	// given
	shoot := &gardenerTypes.Shoot{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: gardenerNamespace},
	}

	// when
	annotate(shoot, runtimeIDAnnotation, "abcd-efgh")
	annotate(shoot, legacyRuntimeIDAnnotation, "abcd-efgh")

	// then
	assertAnnotation(t, shoot, runtimeIDAnnotation, "abcd-efgh")
	assertAnnotation(t, shoot, legacyRuntimeIDAnnotation, "abcd-efgh")
}
