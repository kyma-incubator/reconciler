package db

import (
	"context"
	"github.com/avast/retry-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"sync"
)

//TransactionAwareDatabaseContainerTestSuite manages a test suiteSpec that handles a transaction-enabled connection.
// It can open a connection for you and will roll it back once the suiteSpec is finished.
// You can enable non-isolated runs (not preferred, edge cases only) by enabling CommitAfterExecution. It is possible
// to then also enable SchemaResetOnSetup to make sure that you are working from a clean database. You are able to
// change the isolation level to per-method to ensure a rollback occurs after every method. The connections in this
// test suiteSpec are created lazily, so using it's provided TxConnection will only cause a connection to the database if the test actually
// establishes the connection. In case this is important for benchmarks, retrieve the connection first to make it does not
// influence your results. This suite is based on a container runtime which will take care of the provisioned data container
type TransactionAwareDatabaseContainerTestSuite struct {
	context.Context
	suite.Suite

	ContainerRuntime
	testcontainers.LogConsumer

	activeConnectionMu sync.Mutex
	activeConnection   *TxConnection

	connectionResilienceSpecification []retry.Option

	schemaResetOnSetup         bool
	debugLogs                  bool
	commitAfterExecution       bool
	terminateContainerAfterAll bool

	connectionCount  int
	rollbackCount    int
	commitCount      int
	schemaResetCount int
}

func (s *TransactionAwareDatabaseContainerTestSuite) SetupSuite() {
	s.T().Log("START OF TEST SUITE")
	s.T().Logf("schema reset on startup:%v,debug:%v,commit after execution:%v,terminateContainerAfterAll: %v",
		s.schemaResetOnSetup, s.debugLogs, s.commitAfterExecution, s.terminateContainerAfterAll,
	)

	if s.LogConsumer != nil {
		s.ContainerRuntime.FollowOutput(s.LogConsumer)
		s.NoError(s.StartLogProducer(s))
	}

	if s.schemaResetOnSetup {
		errorDuringReset := s.Reset()
		s.NoError(errorDuringReset)
		s.schemaResetCount++
	}
}

func (s *TransactionAwareDatabaseContainerTestSuite) TearDownSuite() {
	if s.activeConnection != nil {
		s.activeConnectionMu.Lock()
		defer s.activeConnectionMu.Unlock()
		s.tearDownConnection()
	}

	if s.terminateContainerAfterAll {
		s.NoError(s.StopLogProducer())
		s.NoError(s.Terminate(s))
	}

	if s.LogConsumer != nil {
		s.NoError(s.ContainerRuntime.StopLogProducer())
	}

	s.T().Log("END OF TEST SUITE")
	s.T().Logf("connectionCount:%v,rollbackCount:%v,commitCount:%v,schemaResetCount:%v",
		s.connectionCount, s.rollbackCount, s.commitCount, s.schemaResetCount,
	)
}

func (s *TransactionAwareDatabaseContainerTestSuite) TxConnection() *TxConnection {
	s.activeConnectionMu.Lock()
	defer s.activeConnectionMu.Unlock()
	if s.activeConnection == nil {
		s.T().Log("first time asking for connection in test suiteSpec, initializing...")
		s.initializeConnection()
	}

	return s.activeConnection
}

func (s *TransactionAwareDatabaseContainerTestSuite) initializeConnection() {
	s.NoError(retry.Do(func() error {
		newConnection, connectionError := s.NewConnection()
		if connectionError != nil {
			return connectionError
		}
		tx, txErr := newConnection.Begin()
		if txErr != nil {
			return txErr
		}
		s.activeConnection = tx
		s.connectionCount++
		return nil
	}, s.connectionResilienceSpecification...))
}

func (s *TransactionAwareDatabaseContainerTestSuite) tearDownConnection() {
	if s.activeConnection != nil {

		if !s.activeConnection.committedOrRolledBack {
			var executionError error

			if s.commitAfterExecution {
				executionError = s.activeConnection.commit()
				s.commitCount++
			} else {
				executionError = s.activeConnection.Rollback()
				s.rollbackCount++
			}

			s.NoError(executionError)
		}

		err := s.activeConnection.Close()
		s.NoError(err)

		s.activeConnection = nil
	}
}
