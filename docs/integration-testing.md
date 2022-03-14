# Integration Testing

Integration testing is the phase in software testing in which individual software modules are combined and tested as a group. 
Integration testing checks whether a system or component complies with specified functional requirements. 
It occurs after unit testing and before system testing. In integration testing, we take modules that have been unit tested, and group them in larger aggregates. These aggregates are tested according to a defined integration test plan. After successful integration testing, the integrated system is ready for system testing.

To group these integration test aggregates, we use test suites. A test suite is a collection of test cases that show that a software behaves in a specified way.
For each test suite, you should provide the following information:
- Detailed instructions and goals
- The system configuration to be used during testing
- Prerequisite states or step
- Information about the following tests 

## Database Integration Testing (Postgres)

To run our test suite, we use the go library [testify](https://pkg.go.dev/github.com/stretchr/testify/suite), specifically the suite capability. 
With this setup, we can manage the lifecycle of tests run inside a suite with a set of predefined methods (for example, run before or after all or individual tests are executed). 
As a result, we can manage dependent components of the reconciler, in this case, our database.

### How to write a Postgres integration test

To write your own test based on PostgreSQL, use the following helper:
`pkg/db/transaction_aware_database_container_test_suite.go`

This suite is able to manage a container lifecycle of a postgres database with the help of the [testcontainers](https://golang.testcontainers.org/) framework. Thus, we can couple independent database containers with our test suite execution.
Furthermore, we can isolate tests running together with the database.

To write your own tests, you won't have to set up the test suite yourself, but you can use established helpers depending on your use case.

#### Shared container integration tests with automatic rollback

Use this setup if you just want to run some business logic of the reconciler against a postgres-database and within one transaction context. 
The test suite takes care of using a shared container that is reused between test suites with the same isolation needs. It rolls back your changes made in the test instead of committing them automatically.

The reuse can be achieved by "leasing" a container suite from a globally managed map.

> **IMPORTANT**: You must always return the lease of the container suite so that the container is cleaned up correctly with `ReturnLeasedSharedContainerTestSuite(*testing.T, settings ContainerSettings)`.

```go
package random

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/suite"
	"testing"
)

type DbTestSuite struct{ *db.ContainerTestSuite }

func TestDbSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, &DbTestSuite{db.LeaseSharedContainerTestSuite(
		t, db.DefaultSharedContainerSettings, true, false
	)})
	db.ReturnLeasedSharedContainerTestSuite(t, db.DefaultSharedContainerSettings)
}
```

#### Fully Isolated Integration Tests

If you want a dedicated container for your test (for example, because of special requirements or if you want to have committed changes for something like a collision detection), you can use `IsolatedContainerTestSuite(t *testing.T, debug bool, settings ContainerSettings)`.
Do not choose this method if you just need simple business logic tests, because it consumes many resources for spinning up a dedicated container.

```go
package random

import (
	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/stretchr/testify/suite"
	"testing"
)

type RandomSuite struct{ *db.ContainerTestSuite }

func TestRandomSuite(t *testing.T) {
	suite.Run(t, &RandomSuite{db.IsolatedContainerTestSuite(
		t, true, db.DefaultSharedContainerSettings, true
	)})
}

func (s *RandomSuite) TestAmazingRandomStuff() {
	conn := s.TxConnection()

	s.NoError(conn.Ping())

	_, err := conn.Exec("CREATE TABLE AMAZING_RANDOM_TABLE (ID INTEGER PRIMARY KEY, V VARCHAR(512))")
	s.NoError(err)
}
```