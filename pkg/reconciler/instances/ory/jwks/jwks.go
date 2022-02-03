package jwks

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"

	"github.com/google/uuid"
	jose "github.com/square/go-jose/v3"
)

const (
	disclaimerKey   = "reconciler.kyma-project.io/managed-by-reconciler-disclaimer"
	disclaimerValue = "DO NOT EDIT - This resource is managed by Kyma.\nAny modifications are discarded and the resource is reverted to the original state."
)

type JWKS struct {
	alg  string
	bits int
}

func newJwks(alg string, bits int) *JWKS {
	return &JWKS{alg, bits}
}

// Get generates a JSON Web Key Set with RSA Signature Algorithm and returns the entire jwks secret.
func Get(name types.NamespacedName, alg string, bits int) (*v1.Secret, error) {
	cfg := newJwks(alg, bits)
	jwksSecret, err := cfg.generateJwksSecret()

	if err != nil {
		return nil, errors.Wrap(err, "Unable to generate key")
	}

	data := map[string][]byte{
		"jwks.json": jwksSecret,
	}

	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name.Name,
			Namespace:   name.Namespace,
			Annotations: map[string]string{disclaimerKey: disclaimerValue},
		},
		Data: data,
	}, nil
}

func (j *JWKS) generateJwksSecret() ([]byte, error) {
	id := uuid.New().String()
	key, err := j.generateRSAKey()
	if err != nil {
		return nil, err
	}
	jwks := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Algorithm:    j.alg,
				Use:          "sig",
				Key:          key,
				KeyID:        id,
				Certificates: []*x509.Certificate{},
			},
		},
	}

	data, err := json.Marshal(jwks)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// generateRSAKey generates keypair for corresponding RSA Signature Algorithm.
func (j *JWKS) generateRSAKey() (crypto.PrivateKey, error) {
	if j.bits == 0 {
		j.bits = 2048
	}
	if j.bits < 2048 {
		return nil, errors.Errorf(`jwks: key size must be at least 2048 bit for algorithm`)
	}
	key, err := rsa.GenerateKey(rand.Reader, j.bits)
	if err != nil {
		return nil, errors.Wrap(err, "jwks: unable to generate RSA key")
	}

	return key, nil
}
