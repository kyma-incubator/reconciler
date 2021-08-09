package progress

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
	switch strings.ToLower(kind) {
	case strings.ToLower(string(Deployment)):
		return Deployment, nil
	case strings.ToLower(string(Pod)):
		return Pod, nil
	case strings.ToLower(string(DaemonSet)):
		return DaemonSet, nil
	case strings.ToLower(string(StatefulSet)):
		return StatefulSet, nil
	case strings.ToLower(string(Job)):
		return Job, nil
	default:
		return "", fmt.Errorf("WatchableResource '%s' is not supported", kind)
	}
}
