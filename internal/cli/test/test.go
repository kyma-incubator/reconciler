package test

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/internal/cli"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/persistency"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func NewTestOptions(t *testing.T) *cli.Options {
	cliOptions := &cli.Options{
		Verbose: true,
	}

	var err error
	cliOptions.Registry, err = persistency.NewRegistry(db.NewTestConnectionFactory(t), true)
	require.NoError(t, err)

	return cliOptions
}

func WaitForFreeTCPSocket(t *testing.T, host string, port int, timeout time.Duration) {
	connectToTCPSocket(t, host, port, false, timeout)
}

func WaitForTCPSocket(t *testing.T, host string, port int, timeout time.Duration) {
	connectToTCPSocket(t, host, port, true, timeout)
}

func connectToTCPSocket(t *testing.T, host string, port int, expectPortAllocated bool, timeout time.Duration) {
	check := time.NewTimer(1 * time.Second)
	destAddr := fmt.Sprintf("%s:%d", host, port)
	for {
		select {
		case <-check.C:
			_, err := net.Dial("tcp", destAddr)
			if expectPortAllocated {
				if err == nil {
					return
				}
			} else {
				if err != nil {
					return
				}
			}
		case <-time.After(timeout):
			if expectPortAllocated {
				t.Logf("Timeout reached: could not open TCP connection to '%s' within %.1f seconds",
					destAddr, timeout.Seconds())
			} else {
				t.Logf("Timeout reached: TCP socket '%s' was not freed up within %.1f seconds",
					destAddr, timeout.Seconds())
			}
			check.Stop()
			t.FailNow()
			return
		}
	}
}

func InitViper(t *testing.T) {
	configFile, err := test.GetConfigFile()
	require.NoError(t, err)

	viper.SetConfigFile(configFile)
	require.NoError(t, viper.ReadInConfig())
	require.NotEmpty(t, viper.ConfigFileUsed())
}
