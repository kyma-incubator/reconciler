package rafter

import (
	"context"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/workspace"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fake "k8s.io/client-go/kubernetes/fake"
)

func TestEnsureRafterSecret(t *testing.T) {

	// test cases
	tests := []struct {
		Name            string
		Values          *rafterValues
		PreCreateSecret bool
		ExpectSecret    bool
	}{
		{
			Name: "Existing secret is set",
			Values: &rafterValues{
				ExistingSecret: "rafter-existing-secret",
			},
			ExpectSecret: false,
		},
		{
			Name:            "Rafter secret is already created",
			PreCreateSecret: true,
			ExpectSecret:    true,
		},
		{
			Name:         "Rafter secret created successfully",
			ExpectSecret: true,
			Values: &rafterValues{
				AccessKey: "access-key",
				SecretKey: "secret-key",
			},
		},
	}

	for _, testCase := range tests {
		test := testCase
		t.Run(test.Name, func(t *testing.T) {
			a := CustomAction{
				name: "ensure-rafter-secret",
			}
			ctx := context.Background()
			fakeClient := fake.NewSimpleClientset()
			var existingUID types.UID

			if test.PreCreateSecret {
				s := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rafterSecretName,
						Namespace: rafterNamespace,
					},
				}
				existingSecret, err := fakeClient.CoreV1().Secrets(rafterNamespace).Create(ctx, s, metav1.CreateOptions{})
				assert.NoError(t, err)
				existingUID = existingSecret.UID
			}

			err := a.ensureRafterSecret(ctx, fakeClient, test.Values)
			assert.NoError(t, err)

			secret, err := fakeClient.CoreV1().Secrets(rafterNamespace).Get(ctx, rafterSecretName, metav1.GetOptions{})

			if !test.ExpectSecret {
				assert.True(t, err != nil && kerrors.IsNotFound(err))
			} else {
				assert.NoError(t, err)
			}
			// we confirm the a new secert was not recreated by checking the secret object UID after running ensureRafterSecret()
			if test.PreCreateSecret && test.ExpectSecret {
				assert.True(t, existingUID == secret.UID)
			}
		})
	}
	//
}

func TestReadRafterControllerValues(t *testing.T) {
	tests := []struct {
		Name       string
		ValuesFile string
		ShouldErr  bool
		Values     *rafterValues
	}{
		{
			Name:       "Successfully read values file",
			ValuesFile: "./test_files/valid-values.yaml",
			Values: &rafterValues{
				ExistingSecret: "",
				AccessKey:      "AKIAIOSFODNN7EXAMPLE",
				SecretKey:      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
		},
		{
			Name:       "Fail to read values file",
			ValuesFile: "./test_files/invalid-values.yaml",
			ShouldErr:  true,
		},
	}

	for _, testCase := range tests {
		test := testCase
		t.Run(test.Name, func(t *testing.T) {
			values, err := readValues(test.ValuesFile)
			if test.ShouldErr {
				assert.Error(t, err)
			} else {
				assert.EqualValues(t, test.Values, values)
			}
		})
	}
}

func TestActionRun(t *testing.T) {
	tests := []struct {
		Name         string
		Version      string
		ExpectSecret bool
	}{
		{
			Name:         "Create new rafter secret",
			Version:      "0.0.0",
			ExpectSecret: true,
		},
		{
			Name:    "Values have existing secret name set",
			Version: "0.0.1",
		},
	}

	for _, testCase := range tests {
		test := testCase
		t.Run(test.Name, func(t *testing.T) {

			fakeContext := newFakeServiceContext()
			customAction := CustomAction{
				name: "test-action",
			}

			err := customAction.Run(test.Version, "", nil, fakeContext)
			assert.NoError(t, err)

			fakeClient, _ := fakeContext.KubeClient.Clientset()
			secret, err := fakeClient.CoreV1().Secrets(rafterNamespace).Get(fakeContext.Context, rafterSecretName, metav1.GetOptions{})

			if test.ExpectSecret {
				assert.NoError(t, err)
				assert.NotNil(t, secret)
			} else {
				assert.True(t, err != nil && kerrors.IsNotFound(err))
			}
		})
	}
}

func newFakeServiceContext() *service.ActionContext {
	mockClient := &mocks.Client{}
	mockClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	// We create './test_files/0.0.0/success.yaml' to trick the
	// WorkspaceFactory into thinking that we don't need to
	// clone the kyma repo. This is a temporary workaround
	// since we can't currently mock WorkspaceFactory.
	fakeFactory := &workspace.Factory{
		StorageDir: "./test_files",
		Logger:     log.NewOptionalLogger(true),
	}

	return &service.ActionContext{
		KubeClient:       mockClient,
		Context:          context.Background(),
		WorkspaceFactory: fakeFactory,
		Logger:           log.NewOptionalLogger(true),
	}
}
