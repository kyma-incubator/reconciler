package cmd

import (
	"fmt"
	"github.com/pkg/errors"
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
	OrphanOperationTimeout   time.Duration
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
		0 * time.Minute, //Orphan timeout
		0 * time.Second, //ClusterReconcileInterval
		false,           //CreateEncyptionKey
	}
}

func (o *Options) Validate() error {
	if o.Port <= 0 || o.Port > 65535 {
		return fmt.Errorf("port %d is out of range 1-65535", o.Port)
	}
	if o.Workers <= 0 {
		return errors.New("amount of workers cannot be <= 0")
	}
	if o.WatchInterval <= 0 {
		return errors.New("defined watch interval is <= 0")
	}
	if o.OrphanOperationTimeout <= 0 {
		return errors.New("defined orphan timeout cannot be <= 0")
	}
	if o.ClusterReconcileInterval <= 0 {
		return errors.New("cluster reconciliation interval cannot be <= 0")
	}
	return ssl.VerifyKeyPair(o.SSLCrt, o.SSLKey)
}
