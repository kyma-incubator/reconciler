package reconciler

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/ssl"
)

type ServerConfig struct {
	Port   int
	SSLCrt string
	SSLKey string
}

func (c *ServerConfig) validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port %d is out of range 1-65535", c.Port)
	}
	return ssl.VerifyKeyPair(c.SSLCrt, c.SSLKey)
}
