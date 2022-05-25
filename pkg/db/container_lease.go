package db

import (
	"context"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"sync"
	"testing"
)

type containerTestSuiteLeaseID string

type leasedSuite struct {
	leaseCount int
	*ContainerTestSuite
}

type containerTestSuiteLeaseHolder struct {
	sync.Mutex
	leases map[containerTestSuiteLeaseID]*leasedSuite
}

type syncedLeasedSharedContainerTestSuiteInstanceHolder struct {
	mu     sync.Mutex
	suites map[string]*containerTestSuiteLeaseID
}

var globalContainerTestSuiteLeases = &containerTestSuiteLeaseHolder{
	sync.Mutex{},
	make(map[containerTestSuiteLeaseID]*leasedSuite),
}
var globalContainerTestSuiteLeaseHolder = &syncedLeasedSharedContainerTestSuiteInstanceHolder{
	mu:     sync.Mutex{},
	suites: make(map[string]*containerTestSuiteLeaseID),
}

func (l *containerTestSuiteLeaseID) acquire() *ContainerTestSuite {
	lh := globalContainerTestSuiteLeases
	lh.Lock()
	defer lh.Unlock()
	lv := *l
	lh.leases[lv].leaseCount++
	return lh.leases[lv].ContainerTestSuite
}

func (l *containerTestSuiteLeaseID) release() {
	lh := globalContainerTestSuiteLeases
	lh.Lock()
	defer lh.Unlock()

	if len(lh.leases) == 0 || lh.leases[*l] == nil {
		return
	}

	lh.leases[*l].leaseCount--

	if lh.leases[*l].leaseCount == 0 {
		terminationErr := lh.leases[*l].ContainerTestSuite.Terminate(context.Background())
		if terminationErr != nil {
			panic(terminationErr)
		}
		lh.leases[*l] = nil
	}
}

func newContainerTestSuiteLease(debug bool, settings ContainerSettings, commitAfterExecution bool) (*containerTestSuiteLeaseID, error) {
	lh := globalContainerTestSuiteLeases
	lh.Lock()
	defer lh.Unlock()

	id := containerTestSuiteLeaseID(uuid.NewString())
	ctx := context.Background()

	var containerRuntime ContainerRuntime
	var containerErr error
	switch settings := settings.(type) {
	case *PostgresContainerSettings:
		containerRuntime, containerErr = RunPostgresContainer(ctx, *settings, debug)
	case PostgresContainerSettings:
		containerRuntime, containerErr = RunPostgresContainer(ctx, settings, debug)
	}
	if containerErr != nil {
		return nil, containerErr
	}

	lh.leases[id] = &leasedSuite{
		0,
		NewUnmanagedContainerTestSuite(ctx, containerRuntime, commitAfterExecution, nil),
	}

	return &id, nil
}

func LeaseSharedContainerTestSuite(t *testing.T, settings ContainerSettings, debug bool, commitAfterExecution bool) *ContainerTestSuite {
	t.Helper()
	test.IntegrationTest(t)
	registerCleanupForLeasedSharedContainerTestSuite(t, DefaultSharedContainerSettings)
	h := globalContainerTestSuiteLeaseHolder
	h.mu.Lock()
	defer h.mu.Unlock()
	hash := settings.id()
	if h.suites[hash] == nil {
		lid, err := newContainerTestSuiteLease(debug, settings, commitAfterExecution)
		if err != nil {
			panic(err)
		}
		h.suites[hash] = lid
	}
	return h.suites[hash].acquire()
}

func registerCleanupForLeasedSharedContainerTestSuite(t *testing.T, settings ContainerSettings) {
	t.Helper()
	t.Cleanup(func() {
		h := globalContainerTestSuiteLeaseHolder
		h.mu.Lock()
		defer h.mu.Unlock()
		hash := settings.id()
		if len(h.suites) == 0 || h.suites[hash] == nil {
			return
		}
		h.suites[hash].release()
	})
}
