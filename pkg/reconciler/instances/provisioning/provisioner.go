package provisioning

import (
	"context"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener"
	"github.com/pkg/errors"
	"time"
)

type provisioner struct {
	gardenerProvisioner gardener.GardenerProvisioner
}

func (p provisioner) ProvisionCluster(context context.Context, cluster keb.GardenerConfig, tenant string, subaccountID *string, clusterId, operationId string) error {
	err := p.gardenerProvisioner.StartProvisioning(cluster, tenant, subaccountID, clusterId, operationId)

	if err != nil {
		return err
	}

	ticker := time.NewTicker(10 * time.Second)
	done := make(chan bool)

	resultChannel := make(chan bool)
	errorChannel := make(chan error)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				status, err := p.gardenerProvisioner.GetStatus(cluster)
				if err == nil {
					if status.Status == gardener.StatusCompletedSuccessfully {
						resultChannel <- true
					}

					if status.Status == gardener.StatusFailed {
						errorChannel <- errors.New(status.Message)
					}
				}
			}
		}
	}()

	for {
		select {
		case <-context.Done():
			done <- true
			return errors.New("provisioning operation not completed: " + context.Err().Error())
		case <-resultChannel:
			done <- true
			return nil
		case err := <-errorChannel:
			done <- true
			return err
		}
	}

}
