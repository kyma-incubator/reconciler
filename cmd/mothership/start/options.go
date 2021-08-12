package cmd

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/internal/cli"
	file "github.com/kyma-incubator/reconciler/pkg/files"
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
	ReconcilersCfgPath       string
}

func NewOptions(o *cli.Options) *Options {
	return &Options{o,
		0,               //Port
		"",              //SSLCrt
		"",              //SSLKEy
		0,               //Workers
		0 * time.Second, //WatchInterval
		0 * time.Second, //ClusterReconcileInterval
		"",              //ReconcilersCfg
	}
}

func (o *Options) Validate() error {
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("Port %d is out of range 1-65535", o.Port)
	}
	if !file.Exists(o.ReconcilersCfgPath) {
		return fmt.Errorf("File with component reconcilers configuration not found (path: %s)", o.ReconcilersCfgPath)
	}
	return ssl.VerifyKeyPair(o.SSLCrt, o.SSLKey)
}
