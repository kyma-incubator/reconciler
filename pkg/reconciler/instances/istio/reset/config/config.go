package config

import (
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

// IstioProxyConfig stores input information for IstioProxyReset.
type IstioProxyConfig struct {
	// ImagePrefix of Istio
	ImagePrefix string

	// ImageVersion of Istio
	ImageVersion string

	// RetriesCount after an unsuccessful attempt
	RetriesCount int

	// DelayBetweenRetries in seconds
	DelayBetweenRetries int

	// SleepAfterPodDeletion to avoid races
	SleepAfterPodDeletion int

	// Timeout in minutes for waiting on status after reset
	Timeout int

	// Kubeclient for k8s cluster operations
	Kubeclient kubernetes.Interface

	// Debug mode
	Debug bool

	// Logger to be used
	Log *zap.SugaredLogger
}
