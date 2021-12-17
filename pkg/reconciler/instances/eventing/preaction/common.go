package preaction

import "time"

const (
	namespace = "kyma-system"

	progressTrackerInterval = 5 * time.Second
	progressTrackerTimeout  = 2 * time.Minute
)
