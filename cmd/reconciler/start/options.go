package cmd

import (
	"fmt"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/ssl"
)

type Options struct {
	*cli.Options
	Port      int
	SSLCrt    string
	SSLKey    string
	Workspace string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o, 0, "", "", "."}
}

func (o *Options) Validate() error {
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("Port %d is out of range 1-65535", o.Port)
	}
	return ssl.VerifyKeyPair(o.SSLCrt, o.SSLKey)
}
