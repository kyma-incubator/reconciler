package db

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type SampleDatabaseIntegrationTestSuite struct {
	TransactionAwareDatabaseIntegrationTestSuite
}

type testCase struct {
	suiteSpec        SampleDatabaseIntegrationTestSuite
	connectionCount  int
	rollbackCount    int
	commitCount      int
	schemaResetCount int
}

func TestDatabaseIntegrationTestSuite(t *testing.T) {
	testCases := []*testCase{
		{
			suiteSpec: SampleDatabaseIntegrationTestSuite{
				TransactionAwareDatabaseIntegrationTestSuite{DebugLogs: true},
			},
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
		},
		{
			suiteSpec: SampleDatabaseIntegrationTestSuite{
				TransactionAwareDatabaseIntegrationTestSuite{DebugLogs: true, MethodLevelIsolation: true},
			},
			connectionCount:  2,
			rollbackCount:    2,
			commitCount:      0,
			schemaResetCount: 0,
		},
		{
			suiteSpec: SampleDatabaseIntegrationTestSuite{
				TransactionAwareDatabaseIntegrationTestSuite{DebugLogs: true},
			},
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
		},
	}

	for _, testCase := range testCases {
		testSuite := &testCase.suiteSpec
		suite.Run(t, testSuite)
		testSuite.Equal(testCase.connectionCount, testSuite.ConnectionCount)
		testSuite.Equal(testCase.rollbackCount, testSuite.RollbackCount)
		testSuite.Equal(testCase.commitCount, testSuite.CommitCount)
		testSuite.Equal(testCase.schemaResetCount, testSuite.SchemaResetCount)
	}
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
