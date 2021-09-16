package service

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDependencyChecker(t *testing.T) {

	depChecker := dependencyChecker{[]string{"a", "b"}}

	require.ElementsMatch(t,
		[]string{"a", "b"},
		depChecker.newDependencyCheck(&reconciler.Reconciliation{
			ComponentsReady: []string{"x", "y", "z"},
		}).Missing)
	require.ElementsMatch(t,
		[]string{"b"},
		depChecker.newDependencyCheck(&reconciler.Reconciliation{
			ComponentsReady: []string{"a", "y", "z"},
		}).Missing)
	require.ElementsMatch(t,
		[]string{},
		depChecker.newDependencyCheck(&reconciler.Reconciliation{
			ComponentsReady: []string{"a", "b", "z"},
		}).Missing)
}
