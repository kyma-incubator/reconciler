package db

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type SampleDatabaseIntegrationTestSuite struct {
	TransactionAwareDatabaseIntegrationTestSuite
}

func TestDatabaseIntegrationTestSuite(t *testing.T) {
	tSuiteLevel := &SampleDatabaseIntegrationTestSuite{
		TransactionAwareDatabaseIntegrationTestSuite{DebugLogs: true},
	}
	suite.Run(t, tSuiteLevel)
	tSuiteLevel.Equal(1, tSuiteLevel.ConnectionCount())
	tSuiteLevel.Equal(1, tSuiteLevel.RollbackCount())
	tSuiteLevel.Equal(0, tSuiteLevel.CommitCount())
	tSuiteLevel.Equal(0, tSuiteLevel.SchemaResetCount())

	tMethodLevel := &SampleDatabaseIntegrationTestSuite{
		TransactionAwareDatabaseIntegrationTestSuite{DebugLogs: true, MethodLevelIsolation: true},
	}
	suite.Run(t, tMethodLevel)
	tMethodLevel.Equal(2, tMethodLevel.ConnectionCount())
	tMethodLevel.Equal(2, tMethodLevel.RollbackCount())
	tMethodLevel.Equal(0, tMethodLevel.CommitCount())
	tMethodLevel.Equal(0, tMethodLevel.SchemaResetCount())
}

func (s *SampleDatabaseIntegrationTestSuite) TestDbConnectivityWithSuiteEnabledTransaction() {
	s.NoError(s.TxConnection().Ping())
}

func (s *SampleDatabaseIntegrationTestSuite) TestDbConnectivitySecondTestWithoutConnection() {
	s.Equal(true, true)
}

func (s *SampleDatabaseIntegrationTestSuite) TestDbConnectivityThirdTest() {
	s.NoError(s.TxConnection().Ping())
}
