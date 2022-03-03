package db

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

type SampleDatabaseDatabaseTestSuite struct {
	TransactionAwareDatabaseContainerTestSuite
}

func TestDatabaseContainerTestSuite(t *testing.T) {
	test.IntegrationTest(t)
	testCases := []struct {
		testCaseName     string
		debug            bool
		migrate          bool
		connectionCount  int
		rollbackCount    int
		commitCount      int
		schemaResetCount int
	}{
		{
			testCaseName:     "Managed Suite Without Method Isolation",
			debug:            false,
			migrate:          false,
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			testCase := testCase
			testSuite := &SampleDatabaseDatabaseTestSuite{
				NewManagedContainerTestSuite(testCase.debug, testCase.migrate, nil).TransactionAwareDatabaseContainerTestSuite,
			}
			suite.Run(tInner, testSuite)
			testSuite.Equal(testCase.connectionCount, testSuite.connectionCount)
			testSuite.Equal(testCase.rollbackCount, testSuite.rollbackCount)
			testSuite.Equal(testCase.commitCount, testSuite.commitCount)
			testSuite.Equal(testCase.schemaResetCount, testSuite.schemaResetCount)
		})
	}
}

func (s *SampleDatabaseDatabaseTestSuite) TestDbConnectivityWithSuiteEnabledTransaction() {
	s.NoError(s.TxConnection().Ping())
}

func (s *SampleDatabaseDatabaseTestSuite) TestDbConnectivitySecondTestWithoutConnection() {
	s.Equal(true, true)
}

func (s *SampleDatabaseDatabaseTestSuite) TestDbConnectivityThirdTest() {
	s.NoError(s.TxConnection().Ping())
}

type SingleContainerSampleDatabaseIntegrationTestSuite struct {
	TransactionAwareDatabaseContainerTestSuite
}

func TestDatabaseTestSuiteSharedRuntime(t *testing.T) {
	test.IntegrationTest(t)
	ctx := context.Background()

	runtime, runtimeErr := RunPostgresContainer(ctx, false, false)
	require.NoError(t, runtimeErr)

	testCases := []struct {
		testCaseName     string
		connectionCount  int
		rollbackCount    int
		commitCount      int
		schemaResetCount int
	}{
		{
			testCaseName:     "Unmanaged Suite With Method Isolation",
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			testCase := testCase
			tInner.Parallel()
			testSuite := &SingleContainerSampleDatabaseIntegrationTestSuite{
				NewUnmanagedContainerTestSuite(ctx, runtime, nil).TransactionAwareDatabaseContainerTestSuite,
			}
			suite.Run(tInner, testSuite)
			testSuite.Equal(testCase.connectionCount, testSuite.connectionCount)
			testSuite.Equal(testCase.rollbackCount, testSuite.rollbackCount)
			testSuite.Equal(testCase.commitCount, testSuite.commitCount)
			testSuite.Equal(testCase.schemaResetCount, testSuite.schemaResetCount)
		})
	}

	t.Cleanup(func() {
		require.NoError(t, runtime.Terminate(ctx))
	})
}

func (s *SingleContainerSampleDatabaseIntegrationTestSuite) TestFirstSimplePingTestForSingleContainer() {
	s.NoError(s.TxConnection().Ping())
}

func (s *SingleContainerSampleDatabaseIntegrationTestSuite) TestSecondSimplePingTestForSingleContainer() {
	s.NoError(s.TxConnection().Ping())
}
