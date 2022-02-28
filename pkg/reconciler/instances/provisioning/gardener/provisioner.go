package gardener

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/provisioning/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/mitchellh/mapstructure"

	gardener_types "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v12 "k8s.io/api/core/v1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-incubator/reconciler/pkg/keb"
)

//go:generate mockery -name=Client
type Client interface {
	Create(ctx context.Context, shoot *v1beta1.Shoot, opts v1.CreateOptions) (*v1beta1.Shoot, error)
	Update(ctx context.Context, shoot *v1beta1.Shoot, opts v1.UpdateOptions) (*v1beta1.Shoot, error)
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1beta1.Shoot, error)
}

func NewProvisioner(
	namespace string,
	shootClient Client,
	policyConfigMapName string, maintenanceWindowConfigPath string) *GardenerProvisioner {
	return &GardenerProvisioner{
		namespace:                   namespace,
		shootClient:                 shootClient,
		policyConfigMapName:         policyConfigMapName,
		maintenanceWindowConfigPath: maintenanceWindowConfigPath,
	}
}

type GardenerProvisioner struct {
	namespace                   string
	shootClient                 Client
	policyConfigMapName         string
	maintenanceWindowConfigPath string
}

func (g *GardenerProvisioner) StartProvisioning(cluster keb.GardenerConfig, tenant string, subaccountID *string, clusterId, operationId string) error {
	shootTemplate, err := GardenerConfig(cluster).ToShootTemplate(g.namespace, tenant, util.UnwrapStr(subaccountID), cluster.OidcConfig, cluster.DnsConfig)
	if err != nil {
		return errors.New("failed to convert cluster config to Shoot template")
	}

	region := cluster.Region
	purpose := ""
	if cluster.Purpose != nil {
		purpose = *cluster.Purpose
	}

	if g.shouldSetMaintenanceWindow(purpose) {
		err := g.setMaintenanceWindow(shootTemplate, region)

		if err != nil {
			return errors.New(fmt.Sprint("error setting maintenance window for %s cluster", clusterId))
		}
	}

	// TODO: this annotation needs to be set when runtime is registered in Compass
	//annotate(shootTemplate, runtimeIDAnnotation, cluster.ID)

	annotate(shootTemplate, operationIDAnnotation, operationId)

	// TODO: this annotation needs to be set when runtime is registered in Compass
	//annotate(shootTemplate, legacyRuntimeIDAnnotation, clusterId)
	annotate(shootTemplate, legacyOperationIDAnnotation, operationId)

	if g.policyConfigMapName != "" {
		g.applyAuditConfig(shootTemplate)
	}

	_, err = g.shootClient.Create(context.Background(), shootTemplate, v1.CreateOptions{})

	return err
}

type ShootOperationStatus string

const (
	StatusNotExists             ShootOperationStatus = "NotStarted"
	StatusInProgress            ShootOperationStatus = "InProgress"
	StatusFailed                ShootOperationStatus = "Failed"
	StatusCompletedSuccessfully ShootOperationStatus = "Succeeded"
)

type OperationStatus struct {
	Status  ShootOperationStatus
	Message string
}

func (g *GardenerProvisioner) GetStatus(cluster keb.GardenerConfig) (OperationStatus, error) {
	shoot, k8serr := g.shootClient.Get(context.Background(), cluster.Name, v1.GetOptions{})
	if k8serr != nil {
		if k8serrors.IsNotFound(k8serr) {
			return OperationStatus{
				Status: StatusNotExists,
			}, nil
		}
		return OperationStatus{}, nil
	}

	lastOperation := shoot.Status.LastOperation

	if lastOperation != nil {
		if lastOperation.State == gardener_types.LastOperationStateSucceeded {
			return OperationStatus{
				Status: StatusCompletedSuccessfully,
			}, nil
		}

		if lastOperation.State == gardener_types.LastOperationStateFailed {
			if lastOperation.Type == gardener_types.LastOperationTypeReconcile {
				return OperationStatus{
					Status: StatusFailed,
					// TODO: make sure Desription contains error message
					Message: "reconciliation error: " + lastOperation.Description,
				}, nil
			}

			// TODO: gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper" package was removed, make sure it is still needed
			//if gardencorev1beta1helper.HasErrorCode(shoot.Status.LastErrors, gardener_types.ErrorInfraRateLimitsExceeded) {
			//	return OperationStatus{
			//		Status: StatusFailed,
			//		// TODO: make sure Description contains error message
			//		Message: "reconciliation error: rate limits exceeded",
			//	}, nil
			//}
		}
	}

	return OperationStatus{
		Status: StatusInProgress,
	}, nil
}

func (g *GardenerProvisioner) ClusterExists(cluster keb.GardenerConfig) (bool, error) {
	status, err := g.GetStatus(cluster)
	if err != nil {
		return false, err
	}

	return status.Status == StatusNotExists, nil
}

func (g *GardenerProvisioner) shouldSetMaintenanceWindow(purpose string) bool {
	return g.maintenanceWindowConfigPath != "" && purpose == "production"
}

func (g *GardenerProvisioner) applyAuditConfig(template *gardener_types.Shoot) {
	if template.Spec.Kubernetes.KubeAPIServer == nil {
		template.Spec.Kubernetes.KubeAPIServer = &gardener_types.KubeAPIServerConfig{}
	}

	template.Spec.Kubernetes.KubeAPIServer.AuditConfig = &gardener_types.AuditConfig{
		AuditPolicy: &gardener_types.AuditPolicy{
			ConfigMapRef: &v12.ObjectReference{Name: g.policyConfigMapName},
		},
	}
}

func (g *GardenerProvisioner) setMaintenanceWindow(template *gardener_types.Shoot, region string) error {
	window, err := g.getWindowByRegion(region)

	if err != nil {
		return err
	}

	if !window.isEmpty() {
		setMaintenanceWindow(window, template)
	} else {
		logrus.Warnf("Cannot set maintenance window. Config for region %s is empty", region)
	}
	return nil
}

func setMaintenanceWindow(window TimeWindow, template *gardener_types.Shoot) {
	template.Spec.Maintenance.TimeWindow = &gardener_types.MaintenanceTimeWindow{Begin: window.Begin, End: window.End}
}

func (g *GardenerProvisioner) getWindowByRegion(region string) (TimeWindow, error) {
	data, err := getDataFromFile(g.maintenanceWindowConfigPath, region)

	if err != nil {
		return TimeWindow{}, err
	}

	var window TimeWindow

	mapErr := mapstructure.Decode(data, &window)

	if mapErr != nil {
		return TimeWindow{}, errors.New(fmt.Sprintf("failed to parse map to struct: %s", mapErr.Error()))
	}

	return window, nil
}

type TimeWindow struct {
	Begin string
	End   string
}

func (tw TimeWindow) isEmpty() bool {
	return tw.Begin == "" || tw.End == ""
}

func getDataFromFile(filepath, region string) (interface{}, error) {
	file, err := os.Open(filepath)

	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to open file: %s", err.Error()))
	}

	defer file.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return "", errors.New(fmt.Sprintf("failed to decode json: %s", err.Error()))
	}
	return data[region], nil
}
