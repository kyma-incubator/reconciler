package main

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/compreconciler"
	"k8s.io/client-go/kubernetes"
)

func main() {
	compreconciler.NewComponentReconciler().Reconcile(&MyPreAction{}, nil)
}

type MyPreAction struct {
}

func (x *MyPreAction) Run(version string, kubeClient *kubernetes.Clientset, status *compreconciler.StatusUpdater) error {
	fmt.Println()
	return nil
}
