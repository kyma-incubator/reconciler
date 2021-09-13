package actions

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"

	"github.com/pkg/errors"

	"github.com/google/uuid"
	jose "github.com/square/go-jose/v3"
)

// GenerateSigningKeys generates a JSON Web Key Set for signing.
func GenerateSigningKeys(id, alg string, bits int) (*jose.JSONWebKeySet, error) {
	if id == "" {
		id = uuid.New().String()
	}

	key, err := generate(jose.SignatureAlgorithm(alg), bits)
	if err != nil {
		return nil, err
	}

	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Algorithm:    alg,
				Use:          "sig",
				Key:          key,
				KeyID:        id,
				Certificates: []*x509.Certificate{},
			},
		},
	}, nil
}

// generate generates keypair for corresponding SignatureAlgorithm.
func generate(alg jose.SignatureAlgorithm, bits int) (crypto.PrivateKey, error) {

	if bits == 0 {
		bits = 2048
	}
	if bits < 2048 {
		return nil, errors.Errorf(`jwksx: key size must be at least 2048 bit for algorithm "%s"`, alg)
	}
	key, err := rsa.GenerateKey(rand.Reader, bits)
	return key, errors.Wrapf(err, "jwks: unable to generate key")

}
