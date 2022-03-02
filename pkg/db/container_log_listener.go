package db

import (
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/zap"
)

type ContainerLogListener struct {
	debug bool
	Log   *zap.SugaredLogger
}

func (s *ContainerLogListener) Accept(l testcontainers.Log) {
	if s.debug {
		s.Log.With("containerLogType", l.LogType).Debug(string(l.Content))
	}
}
