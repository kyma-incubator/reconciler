package service

import "github.com/kyma-incubator/reconciler/pkg/reconciler"

type dependencyChecker struct {
	dependencies []string
}

func (cd *dependencyChecker) newDependencyCheck(task *reconciler.Task) *DependencyCheck {
	var missingDeps []string
	for _, compDep := range cd.dependencies {
		found := false
		for _, compReady := range task.ComponentsReady {
			if compReady == compDep { //check if required component is part of the components which are ready
				found = true
				break
			}
		}
		if !found {
			missingDeps = append(missingDeps, compDep)
		}
	}
	return &DependencyCheck{
		Component: task.Component,
		Required:  cd.dependencies,
		Missing:   missingDeps,
	}
}

type DependencyCheck struct {
	Component string
	Required  []string
	Missing   []string
}

func (cd *DependencyCheck) DependencyMissing() bool {
	return len(cd.Missing) > 0
}
