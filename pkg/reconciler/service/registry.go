package service

import (
	"fmt"
)

var reconcilers = make(map[string]*ComponentReconciler)
var dryRun bool

func RegisterReconciler(reconcilerName string, reconciler *ComponentReconciler) {
	reconcilers[reconcilerName] = reconciler
}

func GetReconciler(reconcilerName string) (*ComponentReconciler, error) {
	reconciler, ok := reconcilers[reconcilerName]
	if !ok {
		return nil, fmt.Errorf("component reconciler '%s' not found in reconciler registry", reconcilerName)
	}
	reconciler.EnableDryRun(dryRun)
	return reconciler, nil
}

func RegisteredReconcilers() []string {
	var reconNames []string
	for reconName := range reconcilers {
		reconNames = append(reconNames, reconName)
	}
	return reconNames
}

func EnableReconcilerDryRun() {
	dryRun = true
}
