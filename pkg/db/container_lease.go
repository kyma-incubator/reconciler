package db

import (
	"context"
	"github.com/google/uuid"
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
	suites map[ContainerSettings]*containerTestSuiteLeaseID
}

var globalContainerTestSuiteLeases = &containerTestSuiteLeaseHolder{
	sync.Mutex{},
	make(map[containerTestSuiteLeaseID]*leasedSuite),
}
var globalContainerTestSuiteLeaseHolder = &syncedLeasedSharedContainerTestSuiteInstanceHolder{
	mu:     sync.Mutex{},
	suites: make(map[ContainerSettings]*containerTestSuiteLeaseID),
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

func newContainerTestSuiteLease(debug bool, settings PostgresContainerSettings, commitAfterExecution bool) (*containerTestSuiteLeaseID, error) {
	lh := globalContainerTestSuiteLeases
	lh.Lock()
	defer lh.Unlock()

	id := containerTestSuiteLeaseID(uuid.NewString())
	ctx := context.Background()

	r, err := RunPostgresContainer(ctx, settings, debug)
	if err != nil {
		return nil, err
	}

	lh.leases[id] = &leasedSuite{
		0,
		NewUnmanagedContainerTestSuite(ctx, r, commitAfterExecution, nil),
	}

	return &id, nil
}

func LeaseSharedContainerTestSuite(t *testing.T, settings ContainerSettings, debug bool, commitAfterExecution bool) *ContainerTestSuite {
	t.Helper()
	h := globalContainerTestSuiteLeaseHolder
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.suites[settings] == nil {
		lid, err := newContainerTestSuiteLease(debug, *settings.(*PostgresContainerSettings), commitAfterExecution)
		if err != nil {
			panic(err)
		}
		h.suites[settings] = lid
	}
	return h.suites[settings].acquire()
}

func ReturnLeasedSharedContainerTestSuite(t *testing.T, settings ContainerSettings) {
	t.Helper()
	t.Cleanup(func() {
		h := globalContainerTestSuiteLeaseHolder
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.suites) == 0 || h.suites[settings] == nil {
			return
		}
		h.suites[settings].release()
	})
}
