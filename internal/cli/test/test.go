package test

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/app"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func NewTestOptions(t *testing.T) *cli.Options {
	dbConnFac, err := db.NewTestConnectionFactory()
	require.NoError(t, err)

	cliOptions := &cli.Options{
		Verbose: true,
	}
	cliOptions.Registry, err = app.NewApplicationRegistry(dbConnFac, true)
	require.NoError(t, err)

	return cliOptions
}

func WaitForTCPSocket(t *testing.T, host string, port int, timeout time.Duration) {
	check := time.NewTimer(1 * time.Second)
	destAddr := fmt.Sprintf("%s:%d", host, port)
	for {
		select {
		case <-check.C:
			_, err := net.Dial("tcp", destAddr)
			if err == nil {
				return
			}
		case <-time.After(timeout):
			t.Logf("Timeout reached: could not open TCP connection to '%s' within %.1f seconds",
				destAddr, timeout.Seconds())
			check.Stop()
			t.Fail()
		}
	}
}

func InitViper(t *testing.T) {
	configFile, err := test.GetConfigFile()
	require.NoError(t, err)

	viper.SetConfigFile(configFile)
	require.NoError(t, viper.ReadInConfig())
}
