package occupancy

import "sync"

type WorkerPoolOccupancy struct {
	sync.Mutex
	PoolID         string
	RunningWorkers int
}
