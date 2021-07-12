package cmd

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"

	"github.com/kyma-incubator/reconciler/internal/cli"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
)

type Options struct {
	*cli.Options
	Port   int
	SSLCrt string
	SSLKey string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, 0, "", ""}
}

func (o *Options) Validate() error {
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("Port %d is out of range 1-65535", o.Port)
	}
	if file.Exists(o.SSLCrt) && file.Exists(o.SSLKey) {
		crt, err := ioutil.ReadFile(o.SSLCrt)
		if err != nil {
			return err
		}
		key, err := ioutil.ReadFile(o.SSLKey)
		if err != nil {
			return err
		}
		_, err = tls.X509KeyPair(crt, key)
		if err != nil {
			return errors.Wrap(err,
				fmt.Sprintf("Provided TLS certificate '%s' and key '%s' is invalid", o.SSLCrt, o.SSLCrt))
		}
	}
	return nil
}
