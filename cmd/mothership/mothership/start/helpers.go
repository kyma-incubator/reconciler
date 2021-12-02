package cmd

import (
	"sort"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

func components(cfg model.ClusterConfigurationEntity) []keb.Component {
	componentsLen := len(cfg.Components)
	components := make([]keb.Component, componentsLen)

	for i := 0; i < componentsLen; i++ {
		component := cfg.Components[i]
		if component != nil {
			components[i] = *component
			continue
		}
	}
	return components
}

func filterReconciliationsAfter(time time.Time, reconciliations []keb.Reconciliation) []keb.Reconciliation {
	filtered := []keb.Reconciliation{}
	for i := range reconciliations {
		if reconciliations[i].Created.After(time) {
			filtered = append(filtered, reconciliations[i])
		}
	}
	return filtered
}

func filterReconciliationsBefore(time time.Time, reconciliations []keb.Reconciliation) []keb.Reconciliation {
	filtered := []keb.Reconciliation{}
	for i := range reconciliations {
		if reconciliations[i].Created.Before(time) {
			filtered = append(filtered, reconciliations[i])
		}
	}
	return filtered
}

func filterReconciliationsTail(reconciliations []keb.Reconciliation, l int) []keb.Reconciliation {
	tail := reconciliations
	sort.Slice(tail, func(i, j int) bool {
		return tail[i].Created.After(tail[j].Created)
	})

	if l > len(tail) {
		return tail
	}

	return tail[:l]
}
