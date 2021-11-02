package service

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/stretchr/testify/require"
)

func TestDependencyChecker(t *testing.T) {

	depChecker := dependencyChecker{[]string{"a", "b"}}

	require.ElementsMatch(t,
		[]string{"a", "b"},
		depChecker.newDependencyCheck(&reconciler.Task{
			ComponentsReady: []string{"x", "y", "z"},
		}).Missing)
	require.ElementsMatch(t,
		[]string{"b"},
		depChecker.newDependencyCheck(&reconciler.Task{
			ComponentsReady: []string{"a", "y", "z"},
		}).Missing)
	require.ElementsMatch(t,
		[]string{},
		depChecker.newDependencyCheck(&reconciler.Task{
			ComponentsReady: []string{"a", "b", "z"},
		}).Missing)
}
