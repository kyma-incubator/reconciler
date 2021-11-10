package config

import (
	"time"

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
	DelayBetweenRetries time.Duration

	// Interval for polling ready status after Proxy Reset.
	Interval time.Duration

	// Timeout for waiting on status after reset
	Timeout time.Duration

	// Kubeclient for k8s cluster operations
	Kubeclient kubernetes.Interface

	// Debug mode
	Debug bool

	// Logger to be used
	Log *zap.SugaredLogger
}
