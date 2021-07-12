package main

import (
	"github.com/kyma-incubator/reconciler/pkg/compreconciler"
)

func main() {
	compreconciler.NewComponentReconciler().Reconcile(nil, nil)
}
