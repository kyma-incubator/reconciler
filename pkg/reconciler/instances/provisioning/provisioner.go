package provisioning

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/gardener"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"github.com/vrischmann/envconfig"
	"go.uber.org/zap"
	"io/ioutil"
	restclient "k8s.io/client-go/rest"
	"time"
)

type asyncProvisioner struct {
	gardenerProvisioner gardener.Provisioner
}

const ReconcilerName = "provisioning"

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	reconciler.
		//register reconciler action (custom reconciliation logic). If no custom reconciliation action is provided,
		//the default reconciliation logic provided by reconciler-framework will be used.
		WithReconcileAction(&ProvisioningAction{
			name: "provision-action",
		})
}

func createProvisioner(log *zap.SugaredLogger) (*asyncProvisioner, error) {
	cfg := config{}
	err := envconfig.InitWithPrefix(&cfg, "APP")
	if err == nil {

		log.Infof("Config: %s", cfg.String())

		gardenerNamespace := fmt.Sprintf("garden-%s", cfg.GardenerProject)

		gardenerClient, err := newGardenerClusterConfig(cfg)
		if err == nil {
			gardenerClientSet, err := gardener.NewClient(gardenerClient)
			if err == nil {
				shootClient := gardenerClientSet.Shoots(gardenerNamespace)
				prov := gardener.NewProvisioner(gardenerNamespace, shootClient, "")
				if err == nil {
					return &asyncProvisioner{
						gardenerProvisioner: *prov,
					}, nil
				}
			}
		}
	}
	return nil, err
}

func newGardenerClusterConfig(cfg config) (*restclient.Config, error) {
	rawKubeconfig, err := ioutil.ReadFile(cfg.GardenerKubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gardener Kubeconfig from path %s: %s", cfg.GardenerKubeconfigPath, err.Error())
	}

	gardenerClusterConfig, err := gardener.RestClientConfig(rawKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gardener cluster config: %s", err.Error())
	}
	return gardenerClusterConfig, nil
}

func (p asyncProvisioner) ProvisionOrUpgrade(context context.Context, gardenerConfig keb.GardenerConfig, tenant string, subaccountID *string, clusterId, operationId string) error {
	exists, err := p.gardenerProvisioner.ClusterNotExists(gardenerConfig)
	if err != nil {
		return err
	}

	if !exists {
		return p.provisionCluster(context, gardenerConfig, tenant, subaccountID, clusterId, operationId)
	} else {
		return p.upgradeCluster(context, gardenerConfig, tenant, subaccountID, clusterId, operationId)
	}
}

func (p asyncProvisioner) provisionCluster(context context.Context, gardenerConfig keb.GardenerConfig, tenant string, subaccountID *string, clusterId, operationId string) error {
	err := p.gardenerProvisioner.StartProvisioning(gardenerConfig, tenant, subaccountID, clusterId, operationId)

	if err != nil {
		return err
	}

	// TODO: make the time configurable
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
				status, err := p.gardenerProvisioner.GetStatus(gardenerConfig)
				// TODO: write error to log
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

func (p asyncProvisioner) upgradeCluster(context context.Context, gardenerConfig keb.GardenerConfig, tenant string, subaccountID *string, clusterId, operationId string) error {
	return nil
}
