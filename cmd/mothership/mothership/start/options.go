package cmd

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/ssl"
)

type Options struct {
	*cli.Options
	Port                     int
	SSLCrt                   string
	SSLKey                   string
	Workers                  int
	WatchInterval            time.Duration
	ClusterReconcileInterval time.Duration
	CreateEncyptionKey       bool
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		0,               //Port
		"",              //SSLCrt
		"",              //SSLKey
		0,               //Workers
		0 * time.Second, //WatchInterval
		0 * time.Second, //ClusterReconcileInterval
		false,           //CreateEncyptionKey
	}
}

func (o *Options) Validate() error {
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("port %d is out of range 1-65535", o.Port)
	}
	return ssl.VerifyKeyPair(o.SSLCrt, o.SSLKey)
}
