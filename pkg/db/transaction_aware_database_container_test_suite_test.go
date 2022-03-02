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

type testCase struct {
	testCaseName         string
	debug                bool
	migrate              bool
	methodLevelIsolation bool
	connectionCount      int
	rollbackCount        int
	commitCount          int
	schemaResetCount     int
}

func TestDatabaseContainerTestSuite(t *testing.T) {
	test.IntegrationTest(t)
	testCases := []*testCase{
		{
			testCaseName:         "Managed Suite Without Method Isolation",
			debug:                false,
			migrate:              false,
			methodLevelIsolation: false,
			connectionCount:      1,
			rollbackCount:        1,
			commitCount:          0,
			schemaResetCount:     0,
		},
		{
			testCaseName:         "Managed Suite With Method Isolation",
			debug:                false,
			migrate:              false,
			methodLevelIsolation: true,
			connectionCount:      2,
			rollbackCount:        2,
			commitCount:          0,
			schemaResetCount:     0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			testSuite := &SampleDatabaseDatabaseTestSuite{
				NewManagedContainerTestSuite(testCase.debug, testCase.migrate, testCase.methodLevelIsolation, nil).TransactionAwareDatabaseContainerTestSuite,
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

type singleContainerTestCase struct {
	testCaseName         string
	methodLevelIsolation bool
	connectionCount      int
	rollbackCount        int
	commitCount          int
	schemaResetCount     int
}

type SingleContainerSampleDatabaseIntegrationTestSuite struct {
	TransactionAwareDatabaseContainerTestSuite
}

func TestDatabaseTestSuiteSharedRuntime(t *testing.T) {
	test.IntegrationTest(t)
	ctx := context.Background()

	runtime, runtimeErr := RunPostgresContainer(false, false, ctx)
	require.NoError(t, runtimeErr)

	testCases := []*singleContainerTestCase{
		{
			testCaseName:         "Unmanaged Suite With Method Isolation",
			methodLevelIsolation: false,
			connectionCount:      1,
			rollbackCount:        1,
			commitCount:          0,
			schemaResetCount:     0,
		},
		{
			testCaseName:         "Unmanaged Suite With Method Isolation",
			methodLevelIsolation: true,
			connectionCount:      2,
			rollbackCount:        2,
			commitCount:          0,
			schemaResetCount:     0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testCaseName, func(tInner *testing.T) {
			tInner.Parallel()
			testSuite := &SingleContainerSampleDatabaseIntegrationTestSuite{
				NewUnmanagedContainerTestSuite(runtime, testCase.methodLevelIsolation, nil, ctx).TransactionAwareDatabaseContainerTestSuite,
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
