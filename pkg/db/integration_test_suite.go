package db

import (
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

//TransactionAwareDatabaseIntegrationTestSuite manages a test suite that handles a transaction-enabled connection.
// It automatically opens a connection before the suite is started and will roll it back once the suite is finished.
// You can enable non-isolated runs (not preferred, edge cases only) by enabling CommitAfterExecution. It is possible
// to then also enable SchemaResetOnSetup to make sure that you are working from a clean database. You are able to
// change the isolation level to per-method to ensure a rollback occurs after every method. The connections in this
// test suite are created lazily, so using it's provided TxConnection will only cause a connection to the database if the test actually
// establishes the connection. In case this is important for benchmarks, retrieve the connection first to make it does not
// influence your results
type TransactionAwareDatabaseIntegrationTestSuite struct {
	Log *zap.SugaredLogger
	suite.Suite
	connectionFactory *ConnectionFactory
	activeConnection  *TxConnection

	SchemaResetOnSetup   bool
	DebugLogs            bool
	CommitAfterExecution bool
	MethodLevelIsolation bool

	connectionCount  int
	rollbackCount    int
	commitCount      int
	schemaResetCount int
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) ConnectionCount() int {
	return s.connectionCount
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) RollbackCount() int {
	return s.rollbackCount
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) CommitCount() int {
	return s.commitCount
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) SchemaResetCount() int {
	return s.schemaResetCount
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) SetupSuite() {
	s.Log = log.NewLogger(s.DebugLogs).With("suite", s.Suite.T().Name())

	s.Log.Infof("START OF TEST SUITE")
	s.Log.Debugf("schema reset on startup:%v,debug:%v,commit after execution:%v,method level isolation:%v",
		s.SchemaResetOnSetup, s.DebugLogs, s.CommitAfterExecution, s.MethodLevelIsolation,
	)

	test.IntegrationTest(s.T())
	factory := NewTestConnectionFactory(s.T())
	s.connectionFactory = &factory

	if !s.MethodLevelIsolation && s.SchemaResetOnSetup {
		errorDuringReset := (*s.connectionFactory).Reset()
		s.NoError(errorDuringReset)
	}

}

func (s *TransactionAwareDatabaseIntegrationTestSuite) TearDownSuite() {
	if !s.MethodLevelIsolation && s.activeConnection != nil {
		s.tearDownConnection()
	}

	s.Log.Infof("END OF TEST SUITE")
	s.Log.Debugf("connectionCount:%v,rollbackCount:%v,commitCount:%v,schemaResetCount:%v",
		s.connectionCount, s.rollbackCount, s.commitCount, s.schemaResetCount,
	)
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) BeforeTest(string, string) {
	if s.MethodLevelIsolation && s.SchemaResetOnSetup {
		errorDuringReset := (*s.connectionFactory).Reset()
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
		s.Log.Debugf("first time asking for connection in test suite, initializing...")
		s.initializeConnection()
	}

	return s.activeConnection
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) initializeConnection() {
	newConnection, connectionError := (*s.connectionFactory).NewConnection()
	s.NoError(connectionError)
	tx, txErr := newConnection.Begin()
	s.NoError(txErr)
	s.activeConnection = tx
	s.connectionCount++
}

func (s *TransactionAwareDatabaseIntegrationTestSuite) tearDownConnection() {
	if s.activeConnection != nil {

		if !s.activeConnection.committedOrRolledBack {
			var executionError error

			if s.CommitAfterExecution {
				executionError = s.activeConnection.commit()
				s.commitCount++
				s.Log.Debugf("committed changes from test")
			} else {
				executionError = s.activeConnection.Rollback()
				s.rollbackCount++
				s.Log.Debugf("rolled back changes from test")
			}

			s.NoError(executionError)
		}

		err := (*s.activeConnection).Close()
		s.NoError(err)

		s.activeConnection = nil
	}
}
