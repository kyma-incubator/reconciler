package db

import (
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/zap"
)

func NewConsoleContainerLogListener(debug bool) *ContainerLogListener {
	return &ContainerLogListener{log.NewLogger(debug)}
}

type ContainerLogListener struct {
	*zap.SugaredLogger
}

func (s *ContainerLogListener) Accept(l testcontainers.Log) {
	s.With("containerLogType", l.LogType).Debug(string(l.Content))
}
