package db

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"path/filepath"
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
		connectionCount  int
		rollbackCount    int
		commitCount      int
		schemaResetCount int
		migrations       Migrations
	}{
		{
			testCaseName:     "Managed Suite Without Method Isolation and Migrations",
			debug:            false,
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
			migrations:       NoMigrations,
		},
		{
			testCaseName:     "Managed Suite Without Method Isolation and with Migrations",
			debug:            false,
			connectionCount:  1,
			rollbackCount:    0,
			commitCount:      1,
			schemaResetCount: 1,
			migrations:       Migrations(filepath.Join("..", "..", "configs", "db", "postgres")),
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			testSuite := &SampleDatabaseDatabaseTestSuite{
				NewManagedContainerTestSuite(testCase.debug, testCase.migrations, nil).TransactionAwareDatabaseContainerTestSuite,
			}
			if testCase.schemaResetCount > 0 {
				testSuite.schemaResetOnSetup = true
			}
			if testCase.commitCount > 0 {
				testSuite.commitAfterExecution = true
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

	runtime, runtimeErr := RunPostgresContainer(ctx, NoMigrations, false)
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
		testCase := testCase
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
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
