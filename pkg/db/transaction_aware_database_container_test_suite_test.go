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
	*TransactionAwareDatabaseContainerTestSuite
}

func TestDatabaseContainerTestSuite(t *testing.T) {
	test.IntegrationTest(t)

	noMigrationSettings := testPosgresContainerSettings()

	migrationSettings := testPosgresContainerSettings()
	migrationSettings.Config = MigrationConfig(filepath.Join("..", "..", "configs", "db", "postgres"))

	testCases := []struct {
		testCaseName     string
		debug            bool
		connectionCount  int
		rollbackCount    int
		commitCount      int
		schemaResetCount int
		settings         PostgresContainerSettings
	}{
		{
			testCaseName:     "Managed Suite Without Method Isolation and MigrationConfig",
			debug:            false,
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
			settings:         noMigrationSettings,
		},
		{
			testCaseName:     "Managed Suite Without Method Isolation and with MigrationConfig",
			debug:            false,
			connectionCount:  1,
			rollbackCount:    0,
			commitCount:      1,
			schemaResetCount: 1,
			settings:         migrationSettings,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			testSuite := &SampleDatabaseDatabaseTestSuite{
				NewManagedContainerTestSuite(testCase.debug, testCase.settings, false, nil).TransactionAwareDatabaseContainerTestSuite,
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

func TestIsolatedContainerTestSuite(t *testing.T) {
	test.IntegrationTest(t)

	noMigrationSettings := testPosgresContainerSettings()

	testCases := []struct {
		testCaseName     string
		debug            bool
		connectionCount  int
		rollbackCount    int
		commitCount      int
		schemaResetCount int
		settings         PostgresContainerSettings
	}{
		{
			testCaseName:     "Isolated Suite",
			debug:            false,
			connectionCount:  1,
			rollbackCount:    1,
			commitCount:      0,
			schemaResetCount: 0,
			settings:         noMigrationSettings,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			testSuite := &SampleDatabaseDatabaseTestSuite{
				IsolatedContainerTestSuite(tInner, testCase.debug, testCase.settings, false).TransactionAwareDatabaseContainerTestSuite,
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
	*TransactionAwareDatabaseContainerTestSuite
}

func TestDatabaseTestSuiteSharedRuntime(t *testing.T) {
	test.IntegrationTest(t)
	ctx := context.Background()

	runtime, runtimeErr := RunPostgresContainer(ctx, testPosgresContainerSettings(), false)
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
				NewUnmanagedContainerTestSuite(ctx, runtime, false, nil).TransactionAwareDatabaseContainerTestSuite,
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

func testPosgresContainerSettings() PostgresContainerSettings {
	return PostgresContainerSettings{
		"default-db-shared",
		"postgres:11-alpine",
		NoOpMigrationConfig,
		"127.0.0.1",
		"kyma",
		5432,
		"kyma",
		"kyma",
		false,
		UnittestEncryptionKeyFileConfig,
	}
}
