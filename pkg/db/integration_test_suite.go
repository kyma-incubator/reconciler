package db

import (
	"context"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/zap"
)

//TransactionAwareDatabaseIntegrationTestSuite manages a test suiteSpec that handles a transaction-enabled connection.
// It automatically opens a connection before the suiteSpec is started and will roll it back once the suiteSpec is finished.
// You can enable non-isolated runs (not preferred, edge cases only) by enabling CommitAfterExecution. It is possible
// to then also enable SchemaResetOnSetup to make sure that you are working from a clean database. You are able to
// change the isolation level to per-method to ensure a rollback occurs after every method. The connections in this
// test suiteSpec are created lazily, so using it's provided TxConnection will only cause a connection to the database if the test actually
// establishes the connection. In case this is important for benchmarks, retrieve the connection first to make it does not
// influence your results
type TransactionAwareDatabaseIntegrationTestSuite struct {
	suite.Suite

	context.Context
	Log *zap.SugaredLogger

	ContainerBootstrap
	ConnectionFactory

	activeConnection *TxConnection

	SchemaMigrateOnSetup bool
	SchemaResetOnSetup   bool
	DebugLogs            bool
	CommitAfterExecution bool
	MethodLevelIsolation bool

	ConnectionCount  int
	RollbackCount    int
	CommitCount      int
	SchemaResetCount int
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) Accept(l testcontainers.Log) {
	switch l.LogType {
	case testcontainers.StdoutLog:
		s.Log.Debugf(string(l.Content))
	case testcontainers.StderrLog:
		s.Log.Warnf(string(l.Content))
	}
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) SetupSuite() {
	s.Log = log.NewLogger(s.DebugLogs).With("suiteSpec", s.Suite.T().Name())

	s.Log.Infof("START OF TEST SUITE")
	s.Log.Debugf("schema reset on startup:%v,debug:%v,commit after execution:%v,method level isolation:%v",
		s.SchemaResetOnSetup, s.DebugLogs, s.CommitAfterExecution, s.MethodLevelIsolation,
	)

	test.IntegrationTest(s.T())

	if s.ContainerBootstrap == nil && s.ConnectionFactory == nil {
		s.Log.Debug("no connection factory specified, using default factory")
		s.ContainerBootstrap, s.ConnectionFactory = NewTestPostgresContainer(s.T(), true, s.SchemaMigrateOnSetup)
		s.NoError(s.ContainerBootstrap.StartLogProducer(context.Background()))
		s.ContainerBootstrap.FollowOutput(s)
	}

	if !s.MethodLevelIsolation && s.SchemaResetOnSetup {
		errorDuringReset := s.Reset()
		s.NoError(errorDuringReset)
	}

	s.NoError(s.Init(false))
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) TearDownSuite() {
	if !s.MethodLevelIsolation && s.activeConnection != nil {
		s.tearDownConnection()
	}

	s.NoError(s.ContainerBootstrap.Terminate(context.Background()))

	s.Log.Infof("END OF TEST SUITE")
	s.Log.Debugf("connectionCount:%v,rollbackCount:%v,commitCount:%v,schemaResetCount:%v",
		s.ConnectionCount, s.RollbackCount, s.CommitCount, s.SchemaResetCount,
	)
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) BeforeTest(string, string) {
	if s.MethodLevelIsolation && s.SchemaResetOnSetup {
		errorDuringReset := s.Reset()
		s.NoError(errorDuringReset)
	}
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) AfterTest(string, string) {
	if s.MethodLevelIsolation && s.activeConnection != nil {
		s.tearDownConnection()
	}
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) TxConnection() *TxConnection {
	if s.activeConnection == nil {
		s.Log.Debugf("first time asking for connection in test suiteSpec, initializing...")
		s.initializeConnection()
	}

	return s.activeConnection
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) initializeConnection() {
	newConnection, connectionError := s.NewConnection()
	s.NoError(connectionError)
	tx, txErr := newConnection.Begin()
	s.NoError(txErr)
	s.activeConnection = tx
	s.ConnectionCount++
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) tearDownConnection() {
	if s.activeConnection != nil {

		if !s.activeConnection.committedOrRolledBack {
			var executionError error

			if s.CommitAfterExecution {
				executionError = s.activeConnection.commit()
				s.CommitCount++
				s.Log.Debugf("committed changes from test")
			} else {
				executionError = s.activeConnection.Rollback()
				s.RollbackCount++
				s.Log.Debugf("rolled back changes from test")
			}

			s.NoError(executionError)
		}

		err := (*s.activeConnection).Close()
		s.NoError(err)

		s.activeConnection = nil
	}
}
