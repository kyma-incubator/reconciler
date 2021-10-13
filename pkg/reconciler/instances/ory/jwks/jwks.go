package jwks

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/google/uuid"
	jose "github.com/square/go-jose/v3"
)

type jwksPatchJSON struct {
	Op    string             `json:"op"`
	Path  string             `json:"path"`
	Value jwksPatchJSONValue `json:"value"`
}
type jwksPatchJSONValue struct {
	Jwks []byte `json:"jwks.json"`
}

type JWKS struct {
	alg  string
	bits int
}

func newJwks(alg string, bits int) *JWKS {
	return &JWKS{alg, bits}
}

// Get generates a JSON Web Key Set with RSA Signature Algorithm and returns the JSON encoded patch for Ory secret.
func Get(alg string, bits int) ([]byte, error) {
	cfg := newJwks(alg, bits)
	data, err := cfg.generateJwksSecret()
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate key key")
	}

	patchContent := []jwksPatchJSON{{
		Op:   "add",
		Path: "/data",
		Value: jwksPatchJSONValue{
			Jwks: data,
		},
	}}

	patchDataJSON, err := json.Marshal(patchContent)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal key")
	}

	return patchDataJSON, nil
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
