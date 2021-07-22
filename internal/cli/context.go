package cli

import (
	"context"
	"os"
	"os/signal"
)

func NewContext() context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		oscall := <-c
		if oscall == os.Interrupt {
			cancel()
		}
	}()
	return ctx
}
