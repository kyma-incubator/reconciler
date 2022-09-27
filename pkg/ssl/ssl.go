package ssl

import (
	"crypto/tls"
	"fmt"
	"os"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
)

func VerifyKeyPair(sslCrtFile, sslKeyFile string) error {
	if sslCrtFile == "" && sslKeyFile == "" {
		return nil
	}
	if file.Exists(sslCrtFile) && file.Exists(sslKeyFile) {
		crt, err := os.ReadFile(sslCrtFile)
		if err != nil {
			return err
		}
		key, err := os.ReadFile(sslKeyFile)
		if err != nil {
			return err
		}
		_, err = tls.X509KeyPair(crt, key)
		if err != nil {
			return errors.Wrap(err,
				fmt.Sprintf("Provided TLS certificate '%s' and key '%s' is invalid", sslCrtFile, sslKeyFile))
		}
		return nil
	}
	return fmt.Errorf("SSL certificate cannot be verified: either key or certificate file is missing")
}
