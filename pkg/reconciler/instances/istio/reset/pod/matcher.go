package pod

import (
	"context"

	"go.uber.org/zap"

	"github.com/avast/retry-go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockery --name=Matcher --outpkg=mocks --case=underscore
// Matcher of Pod to the Handler.
type Matcher interface {
	// GetHandlersMap by given pods list.
	GetHandlersMap(kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts WaitOptions) map[Handler][]CustomObject
}

// ParentKindMatcher matches Pod to the Handler by the parent kind.
type ParentKindMatcher struct{}

// NewParentKindMatcher creates a new instance of ParentKindMatcher.
func NewParentKindMatcher() *ParentKindMatcher {
	return &ParentKindMatcher{}
}

func (m *ParentKindMatcher) GetHandlersMap(kubeClient kubernetes.Interface, retryOpts []retry.Option, podsList v1.PodList, log *zap.SugaredLogger, debug bool, waitOpts WaitOptions) map[Handler][]CustomObject {
	handlersMap := make(map[Handler][]CustomObject)
	replicaSets := make(map[CustomObject][]CustomObject)

	handlerCfg := handlerCfg{
		kubeClient: kubeClient,
		retryOpts:  retryOpts,
		log:        log,
		debug:      debug,
		waitOpts:   waitOpts,
	}

	noActionHandler := &NoActionHandler{handlerCfg}
	deleteObjectHandler := &DeleteObjectHandler{handlerCfg}
	rolloutHandler := &RolloutHandler{handlerCfg}

	for _, pod := range podsList.Items {
		parentObject := getParentObjectFromOwnerReferences(pod.OwnerReferences)
		podObject := CustomObject{Name: pod.Name, Namespace: pod.Namespace, Kind: pod.Kind}

		switch parentObject.Kind {
		case "":
			handlersMap[noActionHandler] = append(handlersMap[noActionHandler], podObject)
		case "ReplicaSet":
			// ReplicaSets require further processing
			object := CustomObject{Name: parentObject.Name, Namespace: pod.Namespace, Kind: parentObject.Kind}
			replicaSets[object] = append(replicaSets[object], podObject)
		case "ReplicationController":
			handlersMap[deleteObjectHandler] = append(handlersMap[deleteObjectHandler], podObject)
		default:
			object := CustomObject{Name: parentObject.Name, Namespace: pod.Namespace, Kind: parentObject.Kind}
			handlersMap[rolloutHandler] = appendUniqueObject(handlersMap[rolloutHandler], object)
		}
	}

	podsToDelete, parentsToRollout := checkReplicaSets(handlerCfg, replicaSets)
	for _, podToDelete := range podsToDelete {
		handlersMap[deleteObjectHandler] = appendUniqueObject(handlersMap[deleteObjectHandler], podToDelete)
	}
	for _, parentToRollout := range parentsToRollout {
		handlersMap[rolloutHandler] = appendUniqueObject(handlersMap[rolloutHandler], parentToRollout)
	}

	return handlersMap
}

func appendUniqueObject(s []CustomObject, object CustomObject) []CustomObject {
	if !contains(s, object) {
		s = append(s, object)
	}

	return s
}

func contains(customObjects []CustomObject, object CustomObject) bool {
	for _, customObject := range customObjects {
		if customObject == object {
			return true
		}
	}
	return false
}

func checkReplicaSets(handlerCfg handlerCfg, replicaSets map[CustomObject][]CustomObject) (podsToDelete, parentsToRollout []CustomObject) {
	for replicaSet, podObjects := range replicaSets {
		replicaSet, err := handlerCfg.kubeClient.AppsV1().ReplicaSets(replicaSet.Namespace).Get(context.Background(), replicaSet.Name, metav1.GetOptions{})
		if err != nil {
			handlerCfg.log.Error(err)
		}

		replicaSetParentObject := getParentObjectFromOwnerReferences(replicaSet.OwnerReferences)
		switch replicaSetParentObject.Name {
		case "":
			podsToDelete = append(podsToDelete, podObjects...)
		default:
			object := CustomObject{Name: replicaSetParentObject.Name, Namespace: replicaSet.Namespace, Kind: replicaSetParentObject.Kind}
			parentsToRollout = append(parentsToRollout, object)
		}
	}

	return
}
