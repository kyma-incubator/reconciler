package ory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/ory/x/jwksx"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	oryNamespace   = "kyma-system"
	oryChart       = "ory"
	jwksSecretName = "ory-oathkeeper-jwks-secret"
)

//TODO: please implement component specific action logic here
type CustomAction struct {
	name string
}

type jwksConfig struct {
	// JSON Web Key ID to be used, if empty, random will be generated
	ID string

	// Signature Algorithm for JWKS
	Alg string

	//"The key size in bits. If left empty will default to a secure value for the selected algorithm."
	Bits int
}

var (
	jwksSecretNamespacedName = types.NamespacedName{Name: jwksSecretName, Namespace: oryNamespace}
)

func (a *CustomAction) Run(version, profile string, config []reconciler.Configuration, context *service.ActionContext) error {

	kubeClient, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve native Kubernetes GO client")
	}

	jwks := jwksConfig{
		ID:   "",
		Alg:  "RS256",
		Bits: 0,
	}

	data, err := generateJwks(jwks)
	if err != nil {
		log.Fatal(err)
	}

	name := types.NamespacedName{
		Name:      "ory-oathkeeper-jwks-secret",
		Namespace: "kyma-system",
	}
	secret := prepareSecret(name, data)

	if err := a.ensureOrySecret(context.Context, kubeClient, name, secret); err != nil {
		return errors.Wrap(err, "failed to ensure Ory secret")
	}

	context.Logger.Infof("Action '%s' executed (passed version was '%s')", a.name, version)

	return nil
}

func prepareSecret(name types.NamespacedName, data []byte) v1.Secret {
	return v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: map[string][]byte{"jwks.json": data},
	}
}

func (a *CustomAction) ensureOrySecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Get(ctx, name.Name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return createSecret(ctx, client, name, secret)
		}
		return errors.Wrap(err, "failed to get secret")
	}

	return nil
}

// CreateCredsSecret creates a new Ory hydra credentials secret for accessing the database.
func createSecret(ctx context.Context, client kubernetes.Interface, name types.NamespacedName, secret v1.Secret) error {
	_, err := client.CoreV1().Secrets(name.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("failed to create the secret: %s", err)
	}
	log.Printf("%s created", name.String())

	return err
}

func generateJwks(config jwksConfig) ([]byte, error) {

	key, err := jwksx.GenerateSigningKeys(config.ID, config.Alg, config.Bits)
	if err != nil {
		return nil, fmt.Errorf("unable to generate key: %s", err)
	}
	data, err := json.Marshal(key)
	if err != nil {
		log.Fatal(err)
	}

	return data, nil
}
