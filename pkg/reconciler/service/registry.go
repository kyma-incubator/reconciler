package service

import (
	"fmt"
)

var reconcilers map[string]*ComponentReconciler = make(map[string]*ComponentReconciler)

func Register(reconcilerName string, reconciler *ComponentReconciler) {
	reconcilers[reconcilerName] = reconciler
}

func Get(reconcilerName string) (*ComponentReconciler, error) {
	reconciler, ok := reconcilers[reconcilerName]
	if !ok {
		return nil, fmt.Errorf("component reconciler '%s' not found in reconciler-registry", reconcilerName)
	}
	return reconciler, nil
}
