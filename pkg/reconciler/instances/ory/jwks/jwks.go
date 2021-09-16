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

// PreparePatchData generates a JSON Web Key Set with RSA Signature Algorithm and returns the JSON encoded patch for Ory secret.
func PreparePatchData(alg string, bits int) ([]byte, error) {
	data, err := generateJwksSecret(alg, bits)
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

func generateJwksSecret(alg string, bits int) ([]byte, error) {
	id := uuid.New().String()
	key, err := generateRSAKey(bits)
	if err != nil {
		return nil, err
	}
	jwks := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Algorithm:    alg,
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
func generateRSAKey(bits int) (crypto.PrivateKey, error) {
	if bits == 0 {
		bits = 2048
	}
	if bits < 2048 {
		return nil, errors.Errorf(`jwks: key size must be at least 2048 bit for algorithm`)
	}
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, errors.Wrap(err, "jwks: unable to generate RSA key")
	}

	return key, nil
}
