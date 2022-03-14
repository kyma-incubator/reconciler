# Integration Testing

Integration testing is the phase in software testing in which individual software modules are combined and tested as a group. 
Integration testing is conducted to evaluate the compliance of a system or component with specified functional requirements. 
It occurs after unit testing and before system testing. Integration testing takes as its input modules that have been unit tested, groups them in larger aggregates, applies tests defined in an integration test plan to those aggregates, and delivers as its output the integrated system ready for system testing.

To group these integration test aggregates, we make use of test suites. In software development, a test suite, less commonly known as a validation suite, is a collection of test cases that are intended to be used to test a software program to show that it has some specified set of behaviours. 
A test suite often contains detailed instructions or goals for each collection of test cases and information on the system configuration to be used during testing. 
A group of test cases may also contain prerequisite states or steps, and descriptions of the following tests.

## Database Integration Testing (Postgres)

To run our test suite we use the go library [testify](https://pkg.go.dev/github.com/stretchr/testify/suite), specifically the suite capability. 
Compared to regular testing, this enables us to manage lifecycle of tests run inside a suite through a set of predefined methods (for example, run before or after all or individual tests are executed). 
As a result, it enables us to manage dependant components of the reconciler, in this case our database.

### How to write a Postgres Integration Test

To write your own test based on Postgres, there is a helper available that you can use:
`pkg/db/transaction_aware_database_container_test_suite.go`

This suite is able to manage a container lifecycle of a postgres database with the help of the [testcontainers](https://golang.testcontainers.org/) framework, enabling us to couple independent database containers with our test suite execution. 
It provides the ability to isolate tests running together with the database.

To write your own tests, you won't have to set the test suite up yourself, but you can use established helpers depending on your use case.

#### Shared Container Integration Tests with automatic Rollback

This setup is nice if you just want to run some business logic of the reconciler against a postgres-database and within one transaction context. 
The test suite takes care of using a shared container that is reused between suites with the same isolation needs and rolls back your changes made in the test instead of committing them automatically.

The reuse can be achieved by "leasing" a container suite from a globally managed map.

*IMPORTANT: You must always return the Lease of the container suite so that the container is cleaned up correctly with `ReturnLeasedSharedContainerTestSuite(*testing.T, settings ContainerSettings)`*

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
Do not choose this method if you just need simple business logic tests, because this consumes many resources for spinning up a dedicated container.

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