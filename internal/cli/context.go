package cli

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"os"
	"os/signal"
	"time"
)

const (
	exitInterrupted = 2
	exitDone        = 2
	shutdownTimeout = 10 * time.Second
)

func NewContext() context.Context {
	osSignalC := make(chan os.Signal, 1)
	signal.Notify(osSignalC, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-osSignalC: // first signal, cancel context
			cancel()
			logger.NewOptionalLogger(false).Warnf(
				"Received OS interrupt event: stopping execution context")

			time.AfterFunc(shutdownTimeout, func() {
				logger.NewOptionalLogger(false).Warnf(
					"Program didn't stop after %.1f secs waiting period: hard shutdown", shutdownTimeout.Seconds())
				os.Exit(exitInterrupted)

			})
		case <-ctx.Done():
			os.Exit(exitDone)
		}

		<-osSignalC // second signal, hard exit
		logger.NewOptionalLogger(false).Warnf("Received second OS interrupt event: hard shutdown")
		os.Exit(exitInterrupted)
	}()
	return ctx
}
