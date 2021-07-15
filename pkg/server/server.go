package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type Webserver struct {
	Logger     *zap.Logger
	Port       int
	SSLCrtFile string
	SSLKeyFile string
	Router     *mux.Router
	server     *http.Server
}

func (s *Webserver) logger() *zap.Logger {
	if s.Logger == nil {
		s.Logger = zap.NewNop()
	}
	return s.Logger
}

func (s *Webserver) Start(ctx context.Context) error {
	//run webserver within context
	return s.runServer(ctx)
}

func (s *Webserver) runServer(ctx context.Context) error {
	s.logger().Info(fmt.Sprintf("Webserver starting and listening on port %d", s.Port))
	s.startServer(s.Router)
	<-ctx.Done()
	s.logger().Info("Webserver stopping")
	return s.stopServer()
}

func (s *Webserver) startServer(router *mux.Router) {
	//start server
	s.server = &http.Server{Addr: fmt.Sprintf(":%d", s.Port), Handler: router}
	go func() {
		var err error
		if s.SSLCrtFile != "" && s.SSLKeyFile != "" {
			err = s.server.ListenAndServeTLS(s.SSLCrtFile, s.SSLKeyFile)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger().Error(fmt.Sprintf("Webserver startup failed: %s", err))
		}
	}()
}

func (s *Webserver) stopServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	err := s.server.Shutdown(ctx)

	if err == nil {
		s.logger().Info("Webserver gracefully stopped")
	} else {
		s.logger().Error(fmt.Sprintf("Webserver shutdown failed: %s", err))
	}
	return err
}
