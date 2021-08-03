package istio

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"istio.io/api/operator/v1alpha1"
	"istio.io/istio/istioctl/pkg/install/k8sversion"
	v1alpha12 "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
	"istio.io/istio/operator/pkg/cache"
	"istio.io/istio/operator/pkg/controller/istiocontrolplane"
	"istio.io/istio/operator/pkg/helmreconciler"
	"istio.io/istio/operator/pkg/manifest"
	"istio.io/istio/operator/pkg/name"
	"istio.io/istio/operator/pkg/object"
	"istio.io/istio/operator/pkg/util"
	"istio.io/istio/operator/pkg/util/clog"
	"istio.io/istio/operator/pkg/util/progress"
	"istio.io/istio/pkg/config/labels"
	"istio.io/pkg/log"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type InstallAction struct {
	Flags []string //FIXME - retrieved via config?
}

func (a *InstallAction) Run(revision, profile string, config []reconciler.Configuration, helper *service.ActionHelper) error {
	if !labels.IsDNS1123Label(revision) {
		return fmt.Errorf("revision is invalid: %s", revision)
	}

	l := clog.NewConsoleLogger(os.Stdout, os.Stderr, log.RegisterScope("installer", "installer", 0))
	k8sClientset, err := helper.KubeClient.Clientset()
	if err != nil {
		return err
	}
	if err := k8sversion.IsK8VersionSupported(k8sClientset, l); err != nil {
		return fmt.Errorf("check minimum supported Kubernetes version: %v", err)
	}

	k8sConfig := helper.KubeClient.Config()
	crFiles := []string{} //FIXME: taken from workspace?
	_, iop, err := manifest.GenerateConfig(crFiles, a.applyFlagAliases(revision, "path-to-manifests-in-workspace"), true, k8sConfig, l)
	if err != nil {
		return fmt.Errorf("generate config: %v", err)
	}

	k8sClient, err := client.New(k8sConfig, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	iop, err = a.installManifests(iop, true, true, k8sConfig, k8sClient, 5*time.Minute, l)
	if err != nil {
		return fmt.Errorf("failed to install manifests: %v", err)
	}

	return nil
}

func (a *InstallAction) applyFlagAliases(revision, manifestsPath string) []string {
	if manifestsPath != "" {
		a.Flags = append(a.Flags, fmt.Sprintf("installPackagePath=%s", manifestsPath))
	}
	if revision != "" {
		a.Flags = append(a.Flags, fmt.Sprintf("revision=%s", revision))
	}
	return a.Flags
}

func (a *InstallAction) installManifests(iop *v1alpha12.IstioOperator, force bool, dryRun bool, restConfig *rest.Config, client client.Client,
	waitTimeout time.Duration, l clog.Logger) (*v1alpha12.IstioOperator, error) {
	// Needed in case we are running a test through this path that doesn't start a new process.
	cache.FlushObjectCaches()
	opts := &helmreconciler.Options{
		DryRun: dryRun, Log: l, WaitTimeout: waitTimeout, ProgressLog: progress.NewLog(),
		Force: force,
	}
	recon, err := helmreconciler.NewHelmReconciler(client, restConfig, iop, opts)
	if err != nil {
		return iop, err
	}
	status, err := recon.Reconcile()
	if err != nil {
		return iop, fmt.Errorf("errors occurred during operation: %v", err)
	}
	if status.Status != v1alpha1.InstallStatus_HEALTHY {
		return iop, fmt.Errorf("errors occurred during operation")
	}

	opts.ProgressLog.SetState(progress.StateComplete)

	// Save a copy of what was installed as a CR in the cluster under an internal name.
	iop.Name = savedIOPName(iop)
	if iop.Annotations == nil {
		iop.Annotations = make(map[string]string)
	}
	iop.Annotations[istiocontrolplane.IgnoreReconcileAnnotation] = "true"
	iopStr, err := util.MarshalWithJSONPB(iop)
	if err != nil {
		return iop, err
	}

	return iop, saveIOPToCluster(recon, iopStr)
}

func savedIOPName(iop *v1alpha12.IstioOperator) string {
	ret := name.InstalledSpecCRPrefix
	if iop.Name != "" {
		ret += "-" + iop.Name
	}
	if iop.Spec.Revision != "" {
		ret += "-" + iop.Spec.Revision
	}
	return ret
}

func saveIOPToCluster(reconciler *helmreconciler.HelmReconciler, iop string) error {
	obj, err := object.ParseYAMLToK8sObject([]byte(iop))
	if err != nil {
		return err
	}
	return reconciler.ApplyObject(obj.UnstructuredObject(), false)
}
