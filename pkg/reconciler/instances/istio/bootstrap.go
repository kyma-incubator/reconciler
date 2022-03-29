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
	istioctlBinaryPathMaxLen = 12290 // 3 times 4096 (maxpath) + 2 colons (separators)
)

// IstioPerformer instance should be created only once in the Istio Reconciler life.
// Due to current Reconciler limitations - lack of well defined reconciler instances lifetime - we have to initialize it once per reconcile/delete action.
func istioPerformerCreator(istioProxyReset proxy.IstioProxyReset, provider clientset.Provider, name string) bootstrapIstioPerformer {

	res := func(logger *zap.SugaredLogger) (actions.IstioPerformer, error) {
		pathsConfig := os.Getenv(istioctlBinaryPathEnvKey)
		istioctlPaths, err := parsePaths(pathsConfig)
		if err != nil {
			logger.Errorf("Could not create '%s' component reconciler: Error parsing env variable '%s': %s", name, istioctlBinaryPathEnvKey, err.Error())
			return nil, err
		}
		err = ensureFilesExecutable(istioctlPaths, logger)
		if err != nil {
			logger.Errorf("Could not create '%s' component reconciler: One or more istioctl binaries are not executable '%s': %s", name, istioctlBinaryPathEnvKey, err.Error())
			return nil, err
		}

		resolver, err := newDefaultCommanderResolver(istioctlPaths, logger)
		if err != nil {
			logger.Errorf("Could not create '%s' component reconciler: Error creating DefaultCommanderResolver with istioctlPaths '%s': %s", name, istioctlPaths, err.Error())
			return nil, err
		}

		return actions.NewDefaultIstioPerformer(resolver, istioProxyReset, provider), nil
	}
	return res
}

// defaultCommanderResolver provides default runtime wiring for istioctl.ExecutableResolver
// Implements actions.CommanderResolver
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

	dcr.log.Infof("Resolved istioctl binary: Requested istio version: %s, Found: %s", version.String(), istioBinary.Version().String())

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

// parsePaths func parses and validates executable paths. The input must contain a list of full/absolute filesystem paths of binaries, separated by a semicolon character ';'
func parsePaths(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.Errorf("%s env variable is undefined or empty", istioctlBinaryPathEnvKey)
	}
	if len(trimmed) > istioctlBinaryPathMaxLen {
		return nil, fmt.Errorf("%s env variable exceeds the maximum istio path limit of %d characters", istioctlBinaryPathEnvKey, istioctlBinaryPathMaxLen)
	}
	pathDefs := strings.Split(trimmed, ";")
	var res []string
	for _, path := range pathDefs {
		val := strings.TrimSpace(path)
		if val == "" {
			return nil, errors.New("Invalid (empty) path provided")
		}
		res = append(res, val)
	}
	return res, nil
}

func ensureFilesExecutable(paths []string, logger *zap.SugaredLogger) error {
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			return errors.Wrap(err, "Error getting file data")
		}
		mode := stat.Mode()
		logger.Debugf("%s mode: %s", path, mode)
		if (!mode.IsRegular()) || mode.IsDir() {
			return fmt.Errorf("\"%s\" is not a regular file", path)
		}
		if uint32(mode&0111) == 0 {
			logger.Debugf("%s is not executable, will chmod +x", path)
			err := chmodExecutbale(path, logger)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func chmodExecutbale(pathToFile string, logger *zap.SugaredLogger) error {
	var fileMode os.FileMode = 0777
	if err := os.Chmod(pathToFile, fileMode); err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s is not executable or not existing - Failed to change file mode of istioctl binary to: %s", pathToFile, fileMode))
	}
	stat, err := os.Stat(pathToFile)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error getting file data, after changing file mode of %s to %s", pathToFile, fileMode))
	}
	mode := stat.Mode()
	if uint32(mode&0111) == 0 {
		return errors.Wrap(err, fmt.Sprintf("%s is not executable - 'chmod +x' of istioctl binary was not persisted; File mode: %s", pathToFile, fileMode))
	}
	logger.Debugf("%s chmod to: %s", pathToFile, fileMode)
	return nil
}
