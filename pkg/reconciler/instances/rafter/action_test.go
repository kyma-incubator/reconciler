package rafter

import (
	"context"
	"testing"

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
