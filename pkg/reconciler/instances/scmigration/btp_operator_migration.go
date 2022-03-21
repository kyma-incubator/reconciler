package scmigration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/SAP/sap-btp-service-operator/api/v1alpha1"
	"github.com/SAP/sap-btp-service-operator/client/sm"
	"github.com/SAP/sap-btp-service-operator/client/sm/types"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/scmigration/apis/servicecatalog/v1beta1"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/scheme"
)

// It was decided that this reconciler should reuse code from sap-btp-service-operator-migration application
// This file contains copy of unnexported code from the sap-btp-service-operator-migration and improved
// logging with error handling. The migration logic remains unmodified.
// https://github.com/SAP/sap-btp-service-operator-migration/blob/v0.1.2/migrate/migrator.go

const (
	migratedLabel    = "migrated"
	serviceInstances = "serviceinstances"
	serviceBindings  = "servicebindings"
)

type serviceInstancePair struct {
	svcatInstance *v1beta1.ServiceInstance
	smInstance    *types.ServiceInstance
}

type serviceBindingPair struct {
	svcatBinding *v1beta1.ServiceBinding
	smBinding    *types.ServiceBinding
}

type object interface {
	metav1.Object
	runtime.Object
}

type migrator struct {
	SMClient              sm.Client
	SvcatRestClient       *rest.RESTClient
	SapOperatorRestClient *rest.RESTClient
	ClientSet             *kubernetes.Clientset
	ClusterID             string
	Services              map[string]types.ServiceOffering
	Plans                 map[string]types.ServicePlan
	ac                    *service.ActionContext
}

func newMigrator(ac *service.ActionContext) (*migrator, error) {
	ctx := ac.Context
	namespace := ac.Task.Namespace
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(ac.KubeClient.Kubeconfig()))
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, "sap-btp-service-operator", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret %v/sap-btp-service-operator: %w", namespace, err)
	}
	configMap, err := cs.CoreV1().ConfigMaps(namespace).Get(ctx, "sap-btp-operator-config", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %v/sap-btp-operator-config: %w", namespace, err)
	}
	smClient, err := getSMClient(ctx, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate SMClient with secret %v/%v: %w", secret.Namespace, secret.Name, err)
	}
	services, err := getServices(smClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get SM services: %w", err)
	}
	plans, err := getPlans(smClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get SM plans: %w", err)
	}
	migrator := &migrator{
		ac:                    ac,
		SMClient:              smClient,
		SvcatRestClient:       getK8sClient(cfg, svCatGroupName, svCatGroupVersion),
		SapOperatorRestClient: getK8sClient(cfg, operatorGroupName, operatorGroupVersion),
		ClientSet:             cs,
		ClusterID:             configMap.Data["CLUSTER_ID"],
		Services:              services,
		Plans:                 plans,
	}
	return migrator, nil
}

func getPlans(smclient sm.Client) (map[string]types.ServicePlan, error) {
	plans, err := smclient.ListPlans(nil)
	if err != nil {
		return nil, err
	}
	res := make(map[string]types.ServicePlan)
	for _, plan := range plans.ServicePlans {
		res[plan.ID] = plan
	}
	return res, nil
}

func getServices(smclient sm.Client) (map[string]types.ServiceOffering, error) {
	services, err := smclient.ListOfferings(nil)
	if err != nil {
		return nil, err
	}
	res := make(map[string]types.ServiceOffering)
	for _, svc := range services.ServiceOfferings {
		res[svc.ID] = svc
	}
	return res, nil
}

func (m *migrator) migrateBTPOperator() error {
	parameters := &sm.Parameters{
		FieldQuery: []string{
			fmt.Sprintf("context/clusterid eq '%s'", m.ClusterID),
		},
	}

	smInstances, err := m.SMClient.ListInstances(parameters)
	if err != nil {
		return err
	}
	m.ac.Logger.Infof("Fetched %v instances from SM", len(smInstances.ServiceInstances))

	smBindings, err := m.SMClient.ListBindings(parameters)
	if err != nil {
		return err
	}
	m.ac.Logger.Infof("Fetched %v bindings from SM", len(smBindings.ServiceBindings))

	ctx := m.ac.Context
	svcatInstances := v1beta1.ServiceInstanceList{}
	err = m.SvcatRestClient.Get().Namespace("").Resource(serviceInstances).Do(ctx).Into(&svcatInstances)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	m.ac.Logger.Infof("Fetched %v svcat instances from cluster", len(svcatInstances.Items))

	svcatBindings := v1beta1.ServiceBindingList{}
	err = m.SvcatRestClient.Get().Namespace("").Resource(serviceBindings).Do(ctx).Into(&svcatBindings)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	m.ac.Logger.Infof("Fetched %v svcat bindings from cluster", len(svcatBindings.Items))

	m.ac.Logger.Infof("Preparing resources")
	instancesToMigrate := m.getInstancesToMigrate(smInstances, svcatInstances)
	bindingsToMigrate := m.getBindingsToMigrate(smBindings, svcatBindings)
	if len(instancesToMigrate) == 0 && len(bindingsToMigrate) == 0 {
		m.ac.Logger.Infof("no svcat instances or bindings found for migration")
		return nil
	}
	m.ac.Logger.Infof("found %d instances and %d bindings to migrate", len(instancesToMigrate), len(bindingsToMigrate))

	var failuresBuffer bytes.Buffer
	for _, pair := range instancesToMigrate {
		err := m.migrateInstance(pair)
		if err != nil {
			m.ac.Logger.Error(err)
			failuresBuffer.WriteString(err.Error() + "\n")
		}
	}

	for _, pair := range bindingsToMigrate {
		err := m.migrateBinding(pair)
		if err != nil {
			m.ac.Logger.Error(err)
			failuresBuffer.WriteString(err.Error() + "\n")
		}
	}

	if failuresBuffer.Len() == 0 {
		m.ac.Logger.Infof("Migration completed successfully")
	} else {
		m.ac.Logger.Errorf("Migration failures summary: %v", failuresBuffer.String())
		return fmt.Errorf(failuresBuffer.String())
	}
	return nil
}

func (m *migrator) getInstancesToMigrate(smInstances *types.ServiceInstances, svcatInstances v1beta1.ServiceInstanceList) []serviceInstancePair {
	validInstances := make([]serviceInstancePair, 0)
	for _, svcat := range svcatInstances.Items {
		var smInstance *types.ServiceInstance
		for i, instance := range smInstances.ServiceInstances {
			if instance.ID == svcat.Spec.ExternalID {
				smInstance = &smInstances.ServiceInstances[i]
				break
			}
		}
		if smInstance == nil {
			m.ac.Logger.Infof("svcat instance name '%s' id '%s' (%s) not found in SM, skipping it...", svcat.Name, svcat.Spec.ExternalID, svcat.Name)
			continue
		}
		svcInstance := svcat
		validInstances = append(validInstances, serviceInstancePair{
			svcatInstance: &svcInstance,
			smInstance:    smInstance,
		})
	}

	return validInstances
}

func (m *migrator) getBindingsToMigrate(smBindings *types.ServiceBindings, svcatBindings v1beta1.ServiceBindingList) []serviceBindingPair {
	validBindings := make([]serviceBindingPair, 0)
	for _, svcat := range svcatBindings.Items {
		var smBinding *types.ServiceBinding
		for i, binding := range smBindings.ServiceBindings {
			if binding.ID == svcat.Spec.ExternalID {
				smBinding = &smBindings.ServiceBindings[i]
				break
			}
		}
		if smBinding == nil {
			m.ac.Logger.Infof("svcat binding name '%s' id '%s' (%s) not found in SM, skipping it...", svcat.Name, svcat.Spec.ExternalID, svcat.Name)
			continue
		}
		svcBinding := svcat
		validBindings = append(validBindings, serviceBindingPair{
			svcatBinding: &svcBinding,
			smBinding:    smBinding,
		})
	}

	return validBindings
}

func (m *migrator) migrateInstance(pair serviceInstancePair) error {

	m.ac.Logger.Infof("migrating service instance '%s' in namespace '%s' (smID: '%s')", pair.svcatInstance.Name, pair.svcatInstance.Namespace, pair.svcatInstance.Spec.ExternalID)

	//set k8s label
	requestBody := fmt.Sprintf(`{"k8sname": "%s"}`, pair.svcatInstance.Name)
	buffer := bytes.NewBuffer([]byte(requestBody))
	response, err := m.SMClient.Call(http.MethodPut, fmt.Sprintf("/v1/migrate/service_instances/%s", pair.smInstance.ID), buffer, &sm.Parameters{})
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			m.ac.Logger.Errorf("received statusCode %v", response.StatusCode)
		}
		return fmt.Errorf("failed to add k8s label to service instance name: %s, ID: %s", pair.smInstance.Name, pair.smInstance.ID)
	}

	instance := m.getInstanceStruct(pair)
	res := &v1alpha1.ServiceInstance{}
	err = m.SapOperatorRestClient.Post().
		Namespace(pair.svcatInstance.Namespace).
		Resource(serviceInstances).
		Body(instance).
		Do(m.ac.Context).
		Into(res)

	if err = m.ignoreAlreadyMigrated(instance, res, err); err != nil {
		return fmt.Errorf("failed to create service instance: %v", err.Error())
	}

	if !pair.svcatInstance.DeletionTimestamp.IsZero() {
		m.ac.Logger.Infof("svcat instance '%s' is marked for deletion, deleting it from operator", pair.svcatInstance.Name)
		err = m.SapOperatorRestClient.Delete().Name(res.Name).Namespace(res.Namespace).Do(m.ac.Context).Error()
		if err != nil {
			m.ac.Logger.Errorf("failed to delete instance from operator: %v", err.Error())
		}
	}

	pair.svcatInstance.Finalizers = []string{}
	err = m.SvcatRestClient.Put().Name(pair.svcatInstance.Name).Namespace(pair.svcatInstance.Namespace).Resource(serviceInstances).Body(pair.svcatInstance).Do(m.ac.Context).Error()
	if err != nil {
		return fmt.Errorf("failed to delete finalizer from instance '%s'. Error: %v", pair.svcatInstance.Name, err.Error())
	}

	err = m.deleteSvcatResource(res.Name, res.Namespace, serviceInstances)
	if err != nil {
		m.ac.Logger.Infof("failed to delete svcat resource. Error: %v", err.Error())
	}
	m.ac.Logger.Infof("instance migrated successfully")
	return nil
}

func (m *migrator) migrateBinding(pair serviceBindingPair) error {

	m.ac.Logger.Infof("migrating service binding '%s' in namespace '%s' (smID: '%s')", pair.svcatBinding.Name, pair.svcatBinding.Namespace, pair.svcatBinding.Spec.ExternalID)
	secretExists := true
	secret, err := m.ClientSet.CoreV1().Secrets(pair.svcatBinding.Namespace).Get(m.ac.Context, pair.svcatBinding.Spec.SecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.ac.Logger.Infof("Info: secret named '%s' not found for binding", pair.svcatBinding.Spec.SecretName)
			secretExists = false
		} else {
			return fmt.Errorf("failed to get binding's secret, skipping binding migration. Error: %v", err.Error())
		}
	}
	//add k8sname label and save credentials
	requestBody, err := m.getMigrateBindingRequestBody(pair.svcatBinding.Name, secret)
	if err != nil {
		return fmt.Errorf("failed to build request body for migrating instance. Error: %v", err.Error())
	}
	buffer := bytes.NewBuffer([]byte(requestBody))
	response, err := m.SMClient.Call(http.MethodPut, fmt.Sprintf("/v1/migrate/service_bindings/%s", pair.smBinding.ID), buffer, &sm.Parameters{})
	if err != nil || response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add k8s label to service binding name: %s, ID: %s", pair.smBinding.Name, pair.smBinding.ID)
	}

	if secretExists {
		//add 'binding' label to secret
		if secret.Labels == nil {
			secret.Labels = make(map[string]string, 1)
		}
		secret.Labels["binding"] = pair.svcatBinding.Name
		secret, err = m.ClientSet.CoreV1().Secrets(secret.Namespace).Update(m.ac.Context, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to add label to binding. Error: %v", err.Error())
		}
	}

	binding := m.getBindingStruct(pair)
	res := &v1alpha1.ServiceBinding{}
	err = m.SapOperatorRestClient.Post().
		Namespace(binding.Namespace).
		Resource(serviceBindings).
		Body(binding).
		Do(m.ac.Context).
		Into(res)
	if err = m.ignoreAlreadyMigrated(binding, res, err); err != nil {
		return fmt.Errorf("failed to create service binding: %v", err.Error())
	}

	if secretExists {
		//set the new binding as owner reference for the secret
		t := true
		owner := metav1.OwnerReference{
			APIVersion:         res.APIVersion,
			Kind:               res.Kind,
			Name:               res.Name,
			UID:                res.UID,
			Controller:         &t,
			BlockOwnerDeletion: &t,
		}
		secret.OwnerReferences = []metav1.OwnerReference{owner}
		_, err = m.ClientSet.CoreV1().Secrets(secret.Namespace).Update(m.ac.Context, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to set new binding as owner of secret. Error: %v", err.Error())
		}
	}

	if !pair.svcatBinding.DeletionTimestamp.IsZero() {
		m.ac.Logger.Infof("svcat binding '%s' is marked for deletion, deleting it from operator", pair.svcatBinding.Name)
		err = m.SapOperatorRestClient.Delete().Name(res.Name).Namespace(res.Namespace).Do(m.ac.Context).Error()
		if err != nil {
			m.ac.Logger.Infof("failed to delete binding from operator. Error: %v", err.Error())
		}
	}

	//remove finalizer from binding to avoid deletion of the secret
	pair.svcatBinding.Finalizers = []string{}
	err = m.SvcatRestClient.Put().Name(pair.svcatBinding.Name).Namespace(pair.svcatBinding.Namespace).Resource(serviceBindings).Body(pair.svcatBinding).Do(m.ac.Context).Error()
	if err != nil {
		return fmt.Errorf("failed to delete finalizer from binding '%s'. Error: %v", pair.svcatBinding.Name, err.Error())
	}

	err = m.deleteSvcatResource(res.Name, res.Namespace, serviceBindings)
	if err != nil {
		return fmt.Errorf("failed to delete svcat binding. Error: %v", err.Error())
	}
	m.ac.Logger.Infof("binding migrated successfully")
	return nil
}

func (m *migrator) getInstanceStruct(pair serviceInstancePair) *v1alpha1.ServiceInstance {
	plan := m.Plans[pair.smInstance.ServicePlanID]
	service := m.Services[plan.ServiceOfferingID]

	parametersFrom := make([]v1alpha1.ParametersFromSource, 0)
	for _, param := range pair.svcatInstance.Spec.ParametersFrom {
		parametersFrom = append(parametersFrom, v1alpha1.ParametersFromSource{
			SecretKeyRef: &v1alpha1.SecretKeyReference{
				Name: param.SecretKeyRef.Name,
				Key:  param.SecretKeyRef.Key,
			},
		})
	}

	userInfo, err := json.Marshal(pair.svcatInstance.Spec.UserInfo)
	if err != nil {
		m.ac.Logger.Infof("failed to parse user info for instance %s: %v", pair.svcatInstance.Name, err.Error())
	}

	return &v1alpha1.ServiceInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", operatorGroupName, operatorGroupVersion),
			Kind:       "ServiceInstance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pair.svcatInstance.Name,
			Namespace: pair.svcatInstance.Namespace,
			Labels: map[string]string{
				migratedLabel: "true",
			},
			Annotations: map[string]string{
				"original_creation_timestamp": pair.svcatInstance.CreationTimestamp.String(),
				"original_user_info":          string(userInfo)},
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ServicePlanName:     plan.Name,
			ServiceOfferingName: service.Name,
			ExternalName:        pair.smInstance.Name,
			ParametersFrom:      parametersFrom,
			Parameters:          pair.svcatInstance.Spec.Parameters,
		},
	}
}

func (m *migrator) getBindingStruct(pair serviceBindingPair) *v1alpha1.ServiceBinding {
	parametersFrom := make([]v1alpha1.ParametersFromSource, 0)
	for _, param := range pair.svcatBinding.Spec.ParametersFrom {
		parametersFrom = append(parametersFrom, v1alpha1.ParametersFromSource{
			SecretKeyRef: &v1alpha1.SecretKeyReference{
				Name: param.SecretKeyRef.Name,
				Key:  param.SecretKeyRef.Key,
			},
		})
	}

	userInfo, err := json.Marshal(pair.svcatBinding.Spec.UserInfo)
	if err != nil {
		m.ac.Logger.Infof("failed to parse user info for binding %s. Error: %v", pair.svcatBinding.Name, err.Error())
	}

	return &v1alpha1.ServiceBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s", operatorGroupName, operatorGroupVersion),
			Kind:       "ServiceBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pair.svcatBinding.Name,
			Namespace: pair.svcatBinding.Namespace,
			Labels: map[string]string{
				migratedLabel: "true",
			},
			Annotations: map[string]string{
				"original_creation_timestamp": pair.svcatBinding.CreationTimestamp.String(),
				"original_user_info":          string(userInfo)},
		},
		Spec: v1alpha1.ServiceBindingSpec{
			ServiceInstanceName: pair.svcatBinding.Spec.InstanceRef.Name,
			ExternalName:        pair.smBinding.Name,
			ParametersFrom:      parametersFrom,
			Parameters:          pair.svcatBinding.Spec.Parameters,
		},
	}
}

func (m *migrator) ignoreAlreadyMigrated(obj, res object, err error) error {
	if err == nil {
		return nil
	}
	if !errors.IsAlreadyExists(err) {
		return err
	}
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	resource := fmt.Sprintf("%vs", strings.ToLower(kind))
	if err := m.SapOperatorRestClient.Get().Namespace(obj.GetNamespace()).Resource(resource).Name(obj.GetName()).Do(m.ac.Context).Into(res); err != nil {
		return err
	}
	if res.GetLabels()[migratedLabel] != "true" {
		return fmt.Errorf("resource already exists and is missing label %v", migratedLabel)
	}
	return nil
}

func (m *migrator) deleteSvcatResource(resourceName string, resourceNamespace string, resourceType string) error {
	err := m.SapOperatorRestClient.Get().Name(resourceName).Namespace(resourceNamespace).Resource(resourceType).Do(m.ac.Context).Error()
	if err != nil {
		m.ac.Logger.Infof("failed to get the migrated service instance '%s' status, corresponding svcat resource will not be deleted. Error: %v", resourceName, err.Error())
		return err
	}

	//fmt.Println(fmt.Sprintf("deleting svcat resource type '%s' named '%s' in namespace '%s'", resourceType, resourceName, resourceNamespace))
	err = m.SvcatRestClient.Delete().Name(resourceName).Namespace(resourceNamespace).Resource(resourceType).Do(m.ac.Context).Error()
	return err
}

func (m *migrator) getMigrateBindingRequestBody(k8sName string, secret *corev1.Secret) (string, error) {
	var err error
	secretData := []byte("")
	secretDataEncoded := make(map[string]string)
	if secret != nil {
		for k, v := range secret.Data {
			secretDataEncoded[k] = string(v)
		}

		secretData, err = json.Marshal(secretDataEncoded)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf(`
		{
			"k8sname": "%s",
			"credentials": %s
		}`, k8sName, secretData), nil
}
