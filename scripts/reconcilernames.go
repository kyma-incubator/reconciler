package main

import (
	"fmt"
	"strings"

	_ "github.com/kyma-incubator/reconciler/pkg/reconciler/instances"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

func main() {
	recons := service.RegisteredReconcilers()
	recons = remove(recons, "e2etest")
	fmt.Print(strings.Join(recons, ","))
}

func remove(recons []string, recon string) []string {
	idx := -1
	for i := range recons {
		if recons[i] == recon {
			idx = i
			break
		}
	}
	if idx != -1 {
		recons = append(recons[:idx], recons[idx+1:]...)
	}
	return recons
}
