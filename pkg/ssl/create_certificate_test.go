package ssl

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

func TestGenerateCertificate(t *testing.T) {
	type args struct {
		commonName string
		dnsNames   []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			args: args{
				commonName: "test",
				dnsNames:   []string{"test-me-plz"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateCertificate(tt.args.commonName, tt.args.dnsNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateCertificate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			cert, err := tls.X509KeyPair(got[1], got[0])
			if err != nil {
				t.Error(err)
			}

			// Parse the certificate
			parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				t.Error(err)
			}

			actualCommonName := parsedCert.Subject.CommonName
			if parsedCert.Subject.CommonName != tt.args.commonName {
				t.Errorf("GenerateCertificate() commonName = %s, want %s", actualCommonName, tt.args.commonName)
			}
		})
	}
}
