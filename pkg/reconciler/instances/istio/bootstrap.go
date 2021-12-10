package istio

import (
	"fmt"
	"os"
	"strings"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	istioctlBinaryPathEnvKey = "ISTIOCTL_PATH"
	istioctlBinaryPathMaxLen = 12290 //3 times 4096 (maxpath) + 2 colons (separators)
)

//IstioPerformer instance should be created only once in the Istio Reconciler life.
//Due to current Reconciler limitations - lack of well defined reconciler instances lifetime - we have to initialize it once per reconcile/delete action.
func istioPerformerCreator(istioProxyReset proxy.IstioProxyReset, provider clientset.Provider) bootstrapIstioPerformer {

	res := func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
		pathsConfig := os.Getenv(istioctlBinaryPathEnvKey)
		istioctlPaths, err := parsePaths(pathsConfig)
		if err != nil {
			logger.Errorf("Could not create '%s' component reconciler: Error parsing env variable '%s': %s", ReconcilerName, istioctlBinaryPathEnvKey, err.Error())
			return nil, err
		}

		resolver, err := newDefaultCommanderResolver(istioctlPaths, logger)
		if err != nil {
			logger.Errorf("Could not create '%s' component reconciler: Error parsing env variable '%s': %s", ReconcilerName, istioctlBinaryPathEnvKey, err.Error())
			return nil, err
		}

		return actions.NewDefaultIstioPerformer(resolver, istioProxyReset, provider), nil
	}
	return res
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
