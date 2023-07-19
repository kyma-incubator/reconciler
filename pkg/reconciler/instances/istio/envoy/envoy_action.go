package envoy

import (
	"context"
	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/api/networking/v1alpha3"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace string = "istio-system"
	name      string = "kyma-referer"
)

func IsEnvoyFilterPresent(ctx context.Context, k8sClient client.Client) (bool, error) {
	var filter networkingv1alpha3.EnvoyFilter

	err := k8sClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, &filter)
	if k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func CreateEnvoyFilter(ctx context.Context, k8sClient client.Client) error {
	layeredRuntime := map[string]interface{}{
		"layered_runtime": map[string]interface{}{
			"layers": []interface{}{
				map[string]interface{}{
					"name":         name,
					"static_layer": map[string]interface{}{"envoy.reloadable_features.http_allow_partial_urls_in_referer": "false"},
				},
			},
		},
	}

	val, err := structpb.NewStruct(layeredRuntime)
	if err != nil {
		return err
	}

	filter := networkingv1alpha3.EnvoyFilter{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha3.EnvoyFilter{
			ConfigPatches: []*v1alpha3.EnvoyFilter_EnvoyConfigObjectPatch{
				{
					ApplyTo: v1alpha3.EnvoyFilter_BOOTSTRAP,
					Patch: &v1alpha3.EnvoyFilter_Patch{
						Operation: v1alpha3.EnvoyFilter_Patch_MERGE,
						Value:     val,
					},
				},
			},
		},
	}

	err = k8sClient.Create(ctx, &filter)
	if err != nil {
		return err
	}

	return nil
}
