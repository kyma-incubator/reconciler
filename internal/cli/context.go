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
		log := logger.NewLogger(false)

		select {
		case <-osSignalC: // first signal received: cancel context (graceful shutdown)
			cancel()
			log.Warnf(
				"Received OS interrupt event: stopping execution context")

			time.AfterFunc(shutdownTimeout, func() {
				log.Warnf(
					"Program didn't stop after %.1f secs waiting period: hard shutdown", shutdownTimeout.Seconds())
				os.Exit(exitInterrupted)

			})
		case <-ctx.Done():
			os.Exit(exitDone)
		}

		<-osSignalC // second signal received: exit (hard shutdown)
		log.Warnf("Received second OS interrupt event: hard shutdown")
		os.Exit(exitInterrupted)
	}()
	return ctx
}
