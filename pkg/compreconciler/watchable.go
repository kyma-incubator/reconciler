package compreconciler

import (
	"fmt"
	"strings"
)

const (
	Deployment  WatchableResource = "Deployment"
	Pod         WatchableResource = "Pod"
	DaemonSet   WatchableResource = "DaemonSet"
	StatefulSet WatchableResource = "StatefulSet"
	Job         WatchableResource = "Job"
)

type WatchableResource string

func NewWatchableResource(kind string) (WatchableResource, error) {
	switch strings.Title(strings.ToLower(kind)) {
	case string(Deployment):
		return Deployment, nil
	case string(Pod):
		return Pod, nil
	case string(DaemonSet):
		return DaemonSet, nil
	case string(StatefulSet):
		return StatefulSet, nil
	case string(Job):
		return Job, nil
	default:
		return "", fmt.Errorf("WatchableResource '%s' is not supported", kind)
	}
}
