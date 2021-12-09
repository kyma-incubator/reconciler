package istio

import (
	"fmt"
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
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	ReconcilerName           = "istio-configuration"
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
	istioctlBinaryPathMaxLen = 12290 //3 times 4096 (maxpath) + 2 colons (separators)
)

var _skipBootstrap = false

//nolint:gochecknoinits //usage of init() is intended to register reconciler-instances in centralized registry
func init() {
	//TODO: Temporary solution to disable bootstrap in tests.
	if _skipBootstrap {
		return
	}

	log := logger.NewLogger(false)

	log.Debugf("Initializing component reconciler '%s'", ReconcilerName)
	reconciler, err := service.NewComponentReconciler(ReconcilerName)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err)
	}

	pathsConfig := os.Getenv(istioctlBinaryPathEnvKey)
	paths, err := parsePaths(pathsConfig)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: Error parsing env variable '%s': %s", ReconcilerName, istioctlBinaryPathEnvKey, err.Error())
	}

	//istioctlResolver, err := istioctl.NewDefaultIstioctlResolver(paths, istioctl.DefaultVersionChecker{})
	commanderResolver, err := newDefaultCommanderResolver(paths, log)
	if err != nil {
		log.Fatalf("Could not create '%s' component reconciler: %s", ReconcilerName, err.Error())
	}

	gatherer := data.NewDefaultGatherer()
	matcher := pod.NewParentKindMatcher()
	provider := clientset.DefaultProvider{}
	action := reset.NewDefaultPodsResetAction(matcher)
	istioProxyReset := proxy.NewDefaultIstioProxyReset(gatherer, action)
	performer := actions.NewDefaultIstioPerformer(commanderResolver, istioProxyReset, &provider)
	reconciler.WithReconcileAction(NewReconcileAction(performer)).
		WithDeleteAction(NewUninstallAction(performer))
}

//Provides runtime wiring between istioctl.DefaultVersionChecker, istioctl.DefaultIstioctlResolver and istioctl.DefaultCommander
//Implements actions.CommanderResolver
type defaultCommanderResolver struct {
	log                 *zap.SugaredLogger
	paths               []string
	istioBinaryResolver istioctl.ExecutableResolver
}

func (dcr *defaultCommanderResolver) GetCommander(version istioctl.Version) (istioctl.Commander, error) {
	istioBinary, err := dcr.istioBinaryResolver.FindIstioctl(version)
	if err != nil {
		return nil, err
	}

	dcr.log.Debugf("Resolved istioctl binary: Requested: %s, Found: %s", version.String(), istioBinary.Version().String())

	res := istioctl.NewDefaultCommander(*istioBinary)
	return &res, nil
}

func newDefaultCommanderResolver(paths []string, log *zap.SugaredLogger) (actions.CommanderResolver, error) {

	istioBinaryResolver, err := istioctl.NewDefaultIstioctlResolver(paths, istioctl.DefaultVersionChecker{})
	if err != nil {
		return nil, err
	}

	return &defaultCommanderResolver{
		log:                 log,
		paths:               paths,
		istioBinaryResolver: istioBinaryResolver,
	}, nil
}

//The input must contain a list of full/absolute filesystem paths of istioctl binaries.
//Entries must be separated by a colon character ':'
func parsePaths(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.Errorf("No paths defined")
	}
	if len(trimmed) > istioctlBinaryPathMaxLen {
		return nil, errors.New(fmt.Sprintf("%s env variable exceeds the maximum istio path limit of %d characters", istioctlBinaryPathEnvKey, istioctlBinaryPathMaxLen))
	}
	pathdefs := strings.Split(trimmed, ":")
	res := []string{}
	for _, path := range pathdefs {
		val := strings.TrimSpace(path)
		if val == "" {
			return nil, errors.New("Invalid (empty) path provided")
		}

		stat, err := os.Stat(val)
		if err != nil {
			return nil, errors.Wrap(err, "Error getting file data")
		}
		mode := stat.Mode()
		if (!mode.IsRegular()) || mode.IsDir() {
			return nil, errors.New(fmt.Sprintf("\"%s\" is not a regular file", val))
		}
		if uint32(mode&0111) == 0 {
			return nil, errors.New(fmt.Sprintf("\"%s\" is not executable", val))
		}

		res = append(res, val)
	}
	return res, nil
}
