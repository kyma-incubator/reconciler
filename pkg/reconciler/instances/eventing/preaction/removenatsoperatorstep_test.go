package preaction

import (
	"context"
	pmock "github.com/kyma-incubator/reconciler/pkg/reconciler/chart/mocks"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"strings"
	"testing"

	v1 "k8s.io/api/apps/v1"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes/mocks"
	"github.com/stretchr/testify/require"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	manifestString = "---\n# Source: nats/templates/00-prereqs.yaml\napiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: nats-operator\n  # Change to the name of the namespace where to install NATS Operator.\n  # Alternatively, change to \"nats-io\" to perform a cluster-scoped deployment in supported versions.\n  namespace: kyma-system\n---\n# Source: nats/templates/00-prereqs.yaml\napiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: nats-server\n  namespace: kyma-system\n---\n# Source: nats/templates/00-prereqs.yaml\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: nats-operator\nrules:\n# Allow creating CRDs\n- apiGroups:\n  - apiextensions.k8s.io\n  resources:\n  - customresourcedefinitions\n  verbs: [\"get\", \"list\", \"create\", \"update\", \"watch\"]\n\n# Allow all actions on NATS Operator manager CRDs\n- apiGroups:\n  - nats.io\n  resources:\n  - natsclusters\n  - natsserviceroles\n  verbs: [\"*\"]\n\n# Allowed actions on Pods\n- apiGroups: [\"\"]\n  resources:\n  - pods\n  verbs: [\"create\", \"watch\", \"get\", \"patch\", \"update\", \"delete\", \"list\"]\n\n# Allowed actions on Services\n- apiGroups: [\"\"]\n  resources:\n  - services\n  verbs: [\"create\", \"watch\", \"get\", \"patch\", \"update\", \"delete\", \"list\"]\n\n# Allowed actions on Secrets\n- apiGroups: [\"\"]\n  resources:\n  - secrets\n  verbs: [\"create\", \"watch\", \"get\", \"update\", \"delete\", \"list\"]\n\n# Allow all actions on some special subresources\n- apiGroups: [\"\"]\n  resources:\n  - pods/exec\n  - pods/log\n  - serviceaccounts/token\n  - events\n  verbs: [\"*\"]\n\n# Allow listing Namespaces and ServiceAccounts\n- apiGroups: [\"\"]\n  resources:\n  - namespaces\n  - serviceaccounts\n  verbs: [\"list\", \"get\", \"watch\"]\n\n# Allow actions on Endpoints\n- apiGroups: [\"\"]\n  resources:\n  - endpoints\n  verbs: [\"create\", \"watch\", \"get\", \"update\", \"delete\", \"list\"]\n---\n# Source: nats/templates/00-prereqs.yaml\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\nmetadata:\n  name: nats-server\nrules:\n- apiGroups: [\"\"]\n  resources:\n  - nodes\n  verbs: [\"get\"]\n---\n# Source: nats/templates/00-prereqs.yaml\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRoleBinding\nmetadata:\n  name: nats-operator-binding\nroleRef:\n  apiGroup: rbac.authorization.k8s.io\n  kind: ClusterRole\n  name: nats-operator\nsubjects:\n- kind: ServiceAccount\n  name: nats-operator\n  # Change to the name of the namespace where to install NATS Operator.\n  # Alternatively, change to \"nats-io\" to perform a cluster-scoped deployment in supported versions.\n  namespace: kyma-system\n\n# NOTE: When performing multiple namespace-scoped installations, all\n# \"nats-operator\" service accounts (across the different namespaces)\n# MUST be added to this binding.\n#- kind: ServiceAccount\n#  name: nats-operator\n#  namespace: nats-io\n#- kind: ServiceAccount\n#  name: nats-operator\n#  namespace: namespace-2\n#(...)\n---\n# Source: nats/templates/00-prereqs.yaml\napiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRoleBinding\nmetadata:\n  name: nats-server-binding\nroleRef:\n  apiGroup: rbac.authorization.k8s.io\n  kind: ClusterRole\n  name: nats-server\nsubjects:\n- kind: ServiceAccount\n  name: nats-server\n  namespace: kyma-system\n---\n# Source: nats/templates/20-service.yaml\napiVersion: v1\nkind: Service\nmetadata:\n  name: eventing-nats\n  labels: \n    helm.sh/chart: nats-1.0.0\n    app: nats\n    app.kubernetes.io/managed-by: Helm\n    kyma-project.io/dashboard: eventing\nspec:\n  ports:\n  - name: tcp-client\n    port: 4222\n    targetPort: client\n  selector: \n    app: nats\n---\n# Source: nats/templates/10-deployment.yaml\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nats-operator\n  # Change to the name of the namespace where to install NATS Operator.\n  # Alternatively, change to \"nats-io\" to perform a cluster-scoped deployment in supported versions.\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      name: nats-operator\n  template:\n    metadata:\n      annotations:\n        sidecar.istio.io/inject: \"false\"\n      labels:\n        name: nats-operator\n    spec:\n      serviceAccountName: nats-operator\n      containers:\n      - name: nats-operator\n        image: \"eu.gcr.io/kyma-project/nats-operator:c33012c2\"\n        imagePullPolicy: \"IfNotPresent\"\n        args:\n        - nats-operator\n        # Uncomment to perform a cluster-scoped deployment in supported versions.\n        #- --feature-gates=ClusterScoped=true\n        ports:\n        - name: readyz\n          containerPort: 8080\n        env:\n        - name: MY_POD_NAMESPACE\n          valueFrom:\n            fieldRef:\n              fieldPath: metadata.namespace\n        - name: MY_POD_NAME\n          valueFrom:\n            fieldRef:\n              fieldPath: metadata.name\n        readinessProbe:\n          httpGet:\n            path: /readyz\n            port: readyz\n          initialDelaySeconds: 15\n          timeoutSeconds: 3\n---\n# Source: nats/templates/30-destination-rule.yaml\napiVersion: networking.istio.io/v1alpha3\nkind: DestinationRule\nmetadata:\n  name: eventing-nats\nspec:\n  host: eventing-nats.kyma-system.svc.cluster.local\n  trafficPolicy:\n    tls:\n      mode: DISABLE\n---\n# Source: nats/templates/40-cr.yaml\napiVersion: nats.io/v1alpha2\nkind: NatsCluster\nmetadata:\n  name: eventing-nats\nspec:\n  size: 1\n  version: \"2.1.8\"\n  serverImage: \"eu.gcr.io/kyma-project/external/nats\"\n  pod:\n    annotations:\n      sidecar.istio.io/inject: \"false\"\n    labels:\n      helm.sh/chart: nats-1.0.0\n      app: nats\n      app.kubernetes.io/managed-by: Helm\n      kyma-project.io/dashboard: eventing\n    resources:\n      limits:\n        cpu: 20m\n        memory: 64Mi\n      requests:\n        cpu: 5m\n        memory: 16Mi\n  natsConfig:\n    debug: true\n    trace: true\n  template:\n    spec:\n      affinity:\n        podAntiAffinity:\n          preferredDuringSchedulingIgnoredDuringExecution:\n            - podAffinityTerm:\n                labelSelector:\n                  matchLabels:\n                    nats_cluster: eventing-nats\n                topologyKey: kubernetes.io/hostname\n              weight: 100\n"
	//kyma1xVersion  = "1.24.8"
	//kyma2xVersion  = "2.0"
)

func TestDeletingNatsOperatorResources(t *testing.T) {
	action, actionContext, mockProvider, k8sClient, mockedComponentBuilder := testSetup()

	// execute the step
	err := action.Execute(actionContext, actionContext.Logger)
	require.NoError(t, err)

	// ensure the right calls were invoked
	mockProvider.AssertCalled(t, "RenderManifest", mockedComponentBuilder)
	m := []byte(manifestString)
	unstructs, err := kubernetes.ToUnstructured(m, true)
	require.NoError(t, err)
	for _, u := range unstructs {
		if u.GetName() == eventingNats && strings.EqualFold(u.GetKind(), serviceKind) {
			k8sClient.AssertNotCalled(t, "DeleteResource", u.GetKind(), u.GetName(), namespace)
			continue
		}
		k8sClient.AssertCalled(t, "DeleteResource", u.GetKind(), u.GetName(), namespace)
	}
	k8sClient.AssertCalled(t, "DeleteResource", crdPlural, natsOperatorCRDsToDelete[0], namespace)
	k8sClient.AssertCalled(t, "DeleteResource", crdPlural, natsOperatorCRDsToDelete[1], namespace)
	k8sClient.AssertCalled(t, "GetStatefulSet", actionContext.Context, eventingNats, namespace)
}

// todo execute this test, when the check for kyma2x version is available, see the the todo comment from removenatsoperatorstep:Execute()
//func TestSkippingNatsOperatorDeletionFox2x(t *testing.T) {
//	action, actionContext, mockProvider, k8sClient, _ := testSetup(kyma2xVersion)
//
//	// execute the step
//	err := action.Execute(actionContext, actionContext.Logger)
//	require.NoError(t, err)
//
//	mockProvider.AssertNotCalled(t, "RenderManifest", mock.Anything)
//	k8sClient.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
//	k8sClient.AssertNotCalled(t, "DeleteResource", mock.Anything, mock.Anything, mock.Anything)
//	k8sClient.AssertNotCalled(t, "DeleteResource", mock.Anything, mock.Anything, mock.Anything)
//}

func testSetup() (removeNatsOperatorStep, *service.ActionContext, *pmock.Provider, *mocks.Client, *chart.Component) {
	ctx := context.TODO()
	k8sClient := mocks.Client{}
	log := logger.NewLogger(false)

	mockProvider := pmock.Provider{}
	mockManifest := chart.Manifest{
		Manifest: manifestString,
	}
	action := removeNatsOperatorStep{
		kubeClientProvider: func(context *service.ActionContext, logger *zap.SugaredLogger) (kubernetes.Client, error) {
			return &k8sClient, nil
		},
	}
	mockedComponentBuilder := GetResourcesFromVersion(natsOperatorLastVersion, natsSubChartPath)
	mockProvider.On("RenderManifest", mockedComponentBuilder).Return(&mockManifest, nil)

	// mock the delete calls
	k8sClient.On("Clientset").Return(fake.NewSimpleClientset(), nil)
	m := []byte(manifestString)
	unstructs, _ := kubernetes.ToUnstructured(m, true)
	for _, u := range unstructs {
		k8sClient.On(
			"DeleteResource",
			u.GetKind(),
			u.GetName(),
			namespace,
		).Return(nil, nil)
	}
	var statefulSet = &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: eventingNats, Namespace: namespace},
	}
	k8sClient.On(
		"GetStatefulSet",
		ctx,
		eventingNats,
		namespace,
	).Return(statefulSet, nil)
	k8sClient.On(
		"DeleteResource",
		crdPlural,
		natsOperatorCRDsToDelete[0],
		namespace,
	).Return(nil, nil)
	k8sClient.On(
		"DeleteResource",
		crdPlural,
		natsOperatorCRDsToDelete[1],
		namespace,
	).Return(nil, nil)

	actionContext := &service.ActionContext{
		Context:       ctx,
		Logger:        log,
		ChartProvider: &mockProvider,
		Task:          &reconciler.Task{},
	}
	return action, actionContext, &mockProvider, &k8sClient, mockedComponentBuilder
}
