package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"time"
)

const (
	pkBits = 4096
)

// generate self signed certificate with a key
func GenerateCertificate(key, cert io.Writer) error {
	pk, err := rsa.GenerateKey(rand.Reader, pkBits)
	if err != nil {
		return err
	}

	now := time.Now()
	certTpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "connectivity-proxy-smv.kyma-system.svc"},
		NotBefore:             now,
		NotAfter:              now.AddDate(1, 0, 0),
		BasicConstraintsValid: true,
		DNSNames:              []string{"connectivity-proxy-smv.kyma-system.svc"},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// create private key

	keyBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	}

	if err := pem.Encode(key, &keyBlock); err != nil {
		return err
	}

	// create certificate

	certificateBytes, err := x509.CreateCertificate(rand.Reader, &certTpl, &certTpl, &pk.PublicKey, pk)
	if err != nil {
		return err
	}

	pem.Encode(cert, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateBytes,
	})

	if err := pem.Encode(key, &keyBlock); err != nil {
		return err
	}

	return nil
}
