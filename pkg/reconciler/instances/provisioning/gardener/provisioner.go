package gardener

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/mitchellh/mapstructure"

	gardenerTypes "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v12 "k8s.io/api/core/v1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

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
	policyConfigMapName string, maintenanceWindowConfigPath string) *Provisioner {
	return &Provisioner{
		namespace:                   namespace,
		shootClient:                 shootClient,
		policyConfigMapName:         policyConfigMapName,
		maintenanceWindowConfigPath: maintenanceWindowConfigPath,
	}
}

type Provisioner struct {
	namespace                   string
	shootClient                 Client
	policyConfigMapName         string
	maintenanceWindowConfigPath string
}

func (g *Provisioner) StartProvisioning(cluster keb.GardenerConfig, tenant string, subaccountID *string, clusterId, operationId string) error {
	shootTemplate, err := Config(cluster).ToShootTemplate(g.namespace, tenant, unwrapStr(subaccountID), cluster.OidcConfig, cluster.DnsConfig)
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
			return errors.New(fmt.Sprintf("error setting maintenance window for %s cluster", clusterId))
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

func (g *Provisioner) GetStatus(cluster keb.GardenerConfig) (OperationStatus, error) {
	shoot, k8serr := g.shootClient.Get(context.Background(), cluster.Name, v1.GetOptions{})
	if k8serr != nil {
		if k8sErrors.IsNotFound(k8serr) {
			return OperationStatus{
				Status: StatusNotExists,
			}, nil
		}
		return OperationStatus{}, nil
	}

	lastOperation := shoot.Status.LastOperation

	if lastOperation != nil {
		if lastOperation.State == gardenerTypes.LastOperationStateSucceeded {
			return OperationStatus{
				Status: StatusCompletedSuccessfully,
			}, nil
		}

		if lastOperation.State == gardenerTypes.LastOperationStateFailed {
			if lastOperation.Type == gardenerTypes.LastOperationTypeReconcile {
				return OperationStatus{
					Status: StatusFailed,
					// TODO: make sure Description contains error message
					Message: "reconciliation error: " + lastOperation.Description,
				}, nil
			}

			// TODO: gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper" package was removed, make sure it is still needed
			//if gardencorev1beta1helper.HasErrorCode(shoot.Status.LastErrors, gardenerTypes.ErrorInfraRateLimitsExceeded) {
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

func (g *Provisioner) ClusterExists(cluster keb.GardenerConfig) (bool, error) {
	status, err := g.GetStatus(cluster)
	if err != nil {
		return false, err
	}

	return status.Status == StatusNotExists, nil
}

func (g *Provisioner) shouldSetMaintenanceWindow(purpose string) bool {
	return g.maintenanceWindowConfigPath != "" && purpose == "production"
}

func (g *Provisioner) applyAuditConfig(template *gardenerTypes.Shoot) {
	if template.Spec.Kubernetes.KubeAPIServer == nil {
		template.Spec.Kubernetes.KubeAPIServer = &gardenerTypes.KubeAPIServerConfig{}
	}

	template.Spec.Kubernetes.KubeAPIServer.AuditConfig = &gardenerTypes.AuditConfig{
		AuditPolicy: &gardenerTypes.AuditPolicy{
			ConfigMapRef: &v12.ObjectReference{Name: g.policyConfigMapName},
		},
	}
}

func (g *Provisioner) setMaintenanceWindow(template *gardenerTypes.Shoot, region string) error {
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

func setMaintenanceWindow(window TimeWindow, template *gardenerTypes.Shoot) {
	template.Spec.Maintenance.TimeWindow = &gardenerTypes.MaintenanceTimeWindow{Begin: window.Begin, End: window.End}
}

func (g *Provisioner) getWindowByRegion(region string) (TimeWindow, error) {
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

// UnwrapStr returns string value from pointer
func unwrapStr(strPtr *string) string {
	if strPtr == nil {
		return ""
	}
	return *strPtr
}
