package preaction

import (
	"encoding/json"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/action"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/eventing/log"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	"go.uber.org/zap"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
)

const (
	handleDisableJetstream = "handleEnableJetstream"
	enableJetstreamRegExp  = `enableJetStream[\\"]*:[\\"]*(true|false)`
	jsEnabledFlag          = "JS_ENABLED"
	natsContainerName      = "nats"
	statefulSetName        = "eventing-nats"
	chartsVersion          = "main"
	chartsName             = "eventing"
)

type handleEnablingJetstream struct{}

var _ action.Step = &handleEnablingJetstream{}

func newHandleEnablingJetstream() *handleEnablingJetstream {
	return &handleEnablingJetstream{}
}

// Execute removes the Nats StatefulSet in the cases when the jetstream Flag is disabled
// but there are still jetstream resources remaining in the StatefulSet.
func (r *handleEnablingJetstream) Execute(context *service.ActionContext, logger *zap.SugaredLogger) error {

	// prepare Kubernetes clientset
	clientset, err := context.KubeClient.Clientset()
	if err != nil {
		return err
	}

	// decorate logger
	logger = logger.With(log.KeyStep, handleDisableJetstream)
	jetstreamFlag, errorString := fetchJetstreamFlag(context, logger)
	if errorString != "" {
		logger.With(log.KeyReason, errorString).Info("Step skipped")
		return nil
	}

	logger.Infof("Jetstream is flag is set to %v", jetstreamFlag)

	// be sure NO jetstream-related data still remain in the workload if its disabled
	if !jetstreamFlag {
		natsStatefulSet, err := getStatefulSetUsingClientSet(context, clientset, statefulSetName)
		if err != nil {
			return err
		}
		if jetstreamDataExists(natsStatefulSet) {
			logger.Info("Removing Nats StatefulSet for proper Jetstream uninstallation")
			err = clientset.AppsV1().StatefulSets(namespace).Delete(context.Context, statefulSetName, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		} else {
			logger.With(log.KeyReason, "No preparations need to be done, no JS").Info("Step skipped")
		}
	} else {
		logger.With(log.KeyReason, "No preparations need to be done").Info("Step skipped")
	}

	return nil
}

// fetchJetstreamFlag gets the value of ENABLE_JETSTREAM_BACKEND environment var from the eventing controller deployment.
func fetchJetstreamFlag(context *service.ActionContext, logger *zap.SugaredLogger) (bool, string) {
	comp := GetResourcesFromVersion(chartsVersion, chartsName)

	// get the values from eventing/values.yaml
	configurationValues, err := context.ChartProvider.Configuration(comp)
	if err != nil {
		return false, err.Error()
	}

	// the enableJetstreamFlag is stored in .Values.global.features.enableJetstream
	global := configurationValues["global"].(interface{})
	globalsJSON, err2 := json.Marshal(global)
	if err2 != nil {
		return false, err2.Error()
	}

	// the regex should compile and find the necessary flag
	re := regexp.MustCompile(enableJetstreamRegExp)
	if !re.MatchString(string(globalsJSON)) {
		return false, "Cannot find the jetstream Flag in the eventing helm values"
	}

	matches := re.FindStringSubmatch(string(globalsJSON))
	// first element of the array should be the whole regexp match
	// the second - the group match
	if len(matches) != 2 {
		logger.With(log.KeyReason, "Cannot find the jetstream Flag in the eventing helm values").Info("Step skipped")
		return false, "Cannot find the jetstream Flag in the eventing helm values"
	}

	jetstreamFlag, err := strconv.ParseBool(matches[1])
	if err != nil {
		return false, err.Error()
	}
	return jetstreamFlag, ""
}

// getDeployment returns a Kubernetes deployment given its name.
func getStatefulSetUsingClientSet(context *service.ActionContext, clientset kubernetes.Interface, name string) (*v1.StatefulSet, error) {
	statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(context.Context, name, metav1.GetOptions{})
	if err == nil {
		return statefulSet, nil
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return nil, err
}

// jetstreamDataExists checks if the nats-statefulSet contains the jetstream-related data.
func jetstreamDataExists(statefulSet *v1.StatefulSet) bool {
	for _, container := range statefulSet.Spec.Template.Spec.Containers {
		if !strings.EqualFold(container.Name, natsContainerName) {
			continue
		}
		for _, env := range container.Env {
			// JS_ENABLED signatures that js data is still in place
			if strings.EqualFold(env.Name, jsEnabledFlag) {
				return true
			}
		}
	}
	return false
}
