package istio

import (
	"errors"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/pod/reset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	ReconcilerName           = "istio-configuration"
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
)

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	pathsConfig := os.Getenv(istioctlBinaryPathEnvKey)
	paths, err := parsePaths(pathsConfig)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err.Error())
	}

	//istioctlResolver, err := istioctl.NewDefaultIstioctlResolver(paths, istioctl.DefaultVersionChecker{})
	commanderResolver, err := newDefaultCommanderResolver(paths)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err.Error())
	}

	gatherer := data.NewDefaultGatherer()
	matcher := pod.NewParentKindMatcher()
	provider := clientset.DefaultProvider{}
	action := reset.NewDefaultPodsResetAction(matcher)
	istioProxyReset := proxy.NewDefaultIstioProxyReset(gatherer, action)
	performer := actions.NewDefaultIstioPerformer(commanderResolver, istioProxyReset, &provider)
	reconciler.WithReconcileAction(&ReconcileAction{
		performer: performer,
	})
}

//Provides runtime wiring between istioctl.DefaultVersionChecker, istioctl.DefaultIstioctlResolver and istioctl.DefaultCommander
//Implements actions.CommanderResolver
type defaultCommanderResolver struct {
	paths               []string
	istioBinaryResolver istioctl.IstioctlResolver
}

func (dcr *defaultCommanderResolver) GetCommander(version istioctl.Version) (istioctl.Commander, error) {
	istioBinary, err := dcr.istioBinaryResolver.FindIstioctl(version)
	if err != nil {
		return nil, err
	}

	res := istioctl.NewDefaultCommander(*istioBinary)
	return &res, nil
}

func newDefaultCommanderResolver(paths []string) (actions.CommanderResolver, error) {

	istioBinaryResolver, err := istioctl.NewDefaultIstioctlResolver(paths, istioctl.DefaultVersionChecker{})
	if err != nil {
		return nil, err
	}

	return &defaultCommanderResolver{
		paths:               paths,
		istioBinaryResolver: istioBinaryResolver,
	}, nil
}

//The input must contain a list of full/absolute filesystem paths of istioctl binaries.
//Entries must be separated by a colon character ':'
func parsePaths(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.New("No istioctl paths defined")
	}
	pathdefs := strings.Split(trimmed, ":")
	res := []string{}
	for _, path := range pathdefs {
		//TODO: Consider "sanitization": UTF -> ASCII, only allowed characters, etc.
		val := strings.TrimSpace(path)
		if val == "" {
			return nil, errors.New("Invalid (empty) path provided")
		}
		res = append(res, val)
	}
	return res, nil
}
