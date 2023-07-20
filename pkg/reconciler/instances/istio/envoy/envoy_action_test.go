package envoy

import (
	"context"
	"github.com/stretchr/testify/require"
	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func getClientSet(t *testing.T, objects ...client.Object) client.Client {
	err := corev1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = appsv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = networkingv1alpha3.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objects...).Build()
}

func TestIsEnvoyFilterPresent(t *testing.T) {
	type args struct {
		ctx       context.Context
		k8sClient client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "Should return true if there is kyma-referer EnvoyFilter on cluster in namespace istio-system",
			args: args{
				ctx: context.TODO(),
				k8sClient: getClientSet(t, &networkingv1alpha3.EnvoyFilter{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				}),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Should return false if there is kyma-referer EnvoyFilter on cluster not in istio-system namespace",
			args: args{
				ctx: context.TODO(),
				k8sClient: getClientSet(t, &networkingv1alpha3.EnvoyFilter{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				}),
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Should return false if there is no kyma-referer EnvoyFilter on cluster",
			args: args{
				ctx: context.TODO(),
				k8sClient: getClientSet(t, &networkingv1alpha3.EnvoyFilter{
					ObjectMeta: metav1.ObjectMeta{Name: "some-other", Namespace: "default"},
				}),
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsEnvoyFilterPresent(tt.args.ctx, tt.args.k8sClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsEnvoyFilterPresent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsEnvoyFilterPresent() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateEnvoyFilter(t *testing.T) {
	type args struct {
		ctx       context.Context
		k8sClient client.Client
	}
	tests := []struct {
		name                    string
		args                    args
		wantEnvoyFilterGetError bool
		wantErr                 bool
	}{
		{
			name: "Should create EnvoyFilter",
			args: args{
				ctx:       context.TODO(),
				k8sClient: getClientSet(t),
			},
			wantErr:                 false,
			wantEnvoyFilterGetError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateEnvoyFilter(tt.args.ctx, tt.args.k8sClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateEnvoyFilter() error = %v, wantErr %v", err, tt.wantErr)
			}
			var filter networkingv1alpha3.EnvoyFilter
			err = tt.args.k8sClient.Get(tt.args.ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      name,
			}, &filter)
			if (err != nil) != tt.wantEnvoyFilterGetError {
				t.Errorf("CreateEnvoyFilter() error = %v, wantEnvoyFilterError %v", err, tt.wantEnvoyFilterGetError)
			}
		})
	}
}
