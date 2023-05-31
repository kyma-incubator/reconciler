package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

const (
	pkBits = 4096
)

// generate self signed certificate with a key
func GenerateCertificate(commonName string, dnsNames []string) ([2][]byte, error) {
	pk, err := rsa.GenerateKey(rand.Reader, pkBits)
	if err != nil {
		return [2][]byte{}, err
	}

	now := time.Now()
	certTpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now,
		NotAfter:     now.AddDate(1, 0, 0),
		DNSNames:     dnsNames,
	}

	// create private key

	keyBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	}

	key := pem.EncodeToMemory(&keyBlock)

	// create certificate

	certificateBytes, err := x509.CreateCertificate(rand.Reader, &certTpl, &certTpl, &pk.PublicKey, pk)
	if err != nil {
		return [2][]byte{}, err
	}

	certBlock := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateBytes,
	}

	cert := pem.EncodeToMemory(&certBlock)

	return [2][]byte{key, cert}, nil
}
