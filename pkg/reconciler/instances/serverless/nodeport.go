package serverless

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"math/rand"
)

var _ service.Action = ResolveDockerRegistryNodePort{}

const (
	dockerRegistryNodePort = 32_137

	//Available ports according to documentation https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport
	maxNodePort = 32_767
	minNodePort = 30_000
)

const (
	dockerRegistryService      = "serverless-docker-registry"
	dockerRegistryNodePortPath = "global.registryNodePort"
	dockerRegistryPortName     = "http-registry"

	allNamespaces = ""
)

type nodePortFinder func() int32

type ResolveDockerRegistryNodePort struct {
	nodePortFinder
	name string
}

func (n ResolveDockerRegistryNodePort) Run(svcCtx *service.ActionContext) error {
	k8sClient, err := svcCtx.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "while getting clientset")
	}
	svc, err := getService(svcCtx.Context, k8sClient, svcCtx.Task.Namespace, dockerRegistryService)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("while checking if %s service is installed on cluster", dockerRegistryService))
	}

	if svc != nil && svc.Spec.Type == corev1.ServiceTypeNodePort {
		if isDefaultNodePortValue(svc) {
			return nil
		}
		currentNodePort := getNodePort(svc)
		setNodePortOverride(svcCtx.Task.Configuration, dockerRegistryNodePortPath, currentNodePort)
		return nil
	}

	svcs, err := getAllNodePortServices(svcCtx.Context, k8sClient)
	if err != nil {
		return errors.Wrap(err, "while fetching all services from cluster")
	}

	if possibleConflict(svcs) {
		newPort, err := n.drawEmptyPortNumber(svcs)
		if err != nil {
			return errors.Wrap(err, "while drawing available port number")
		}
		setNodePortOverride(svcCtx.Task.Configuration, dockerRegistryNodePortPath, newPort)
	}
	return nil
}

func getNodePort(svc *corev1.Service) int32 {
	for _, port := range svc.Spec.Ports {
		if port.Name == dockerRegistryPortName {
			return port.NodePort
		}
	}
	return dockerRegistryNodePort
}

func getService(ctx context.Context, k8sClient kubernetes.Interface, namespace, name string) (*corev1.Service, error) {
	svc, err := k8sClient.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, fmt.Sprintf("while getting %s servicce", name))
	}
	return svc, nil
}

func isDefaultNodePortValue(svc *corev1.Service) bool {
	ports := svc.Spec.Ports
	for _, port := range ports {
		if port.NodePort == dockerRegistryNodePort {
			return true
		}
	}
	return false
}

func (n *ResolveDockerRegistryNodePort) drawEmptyPortNumber(svcs *corev1.ServiceList) (int32, error) {
	nodePorts := map[int32]struct{}{}
	for _, svc := range svcs.Items {
		for _, port := range svc.Spec.Ports {
			nodePorts[port.NodePort] = struct{}{}
		}
	}

	retries := 100
	var emptyPort int32
	for i := 0; i < retries; i++ {
		possibleEmptyPort := n.nodePortFinder()
		if _, ok := nodePorts[possibleEmptyPort]; !ok {
			emptyPort = possibleEmptyPort
			break
		}
	}
	if emptyPort == 0 {
		return 0, errors.New("couldn't draw available port number, try again")
	}
	return emptyPort, nil
}

func setNodePortOverride(overrides map[string]interface{}, path string, port int32) {
	overrides[path] = port
}

func getAllNodePortServices(ctx context.Context, k8sClient kubernetes.Interface) (*corev1.ServiceList, error) {
	svcs, err := k8sClient.CoreV1().Services(allNamespaces).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "while getting list of all services")
	}
	nodePortSvcs := &corev1.ServiceList{}
	for _, svc := range svcs.Items {
		if svc.Spec.Type == corev1.ServiceTypeNodePort {
			nodePortSvcs.Items = append(nodePortSvcs.Items, svc)
		}
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, port := range svc.Spec.Ports {
				if port.NodePort != 0 {
					nodePortSvcs.Items = append(nodePortSvcs.Items, svc)
					break
				}
			}
		}
	}
	return nodePortSvcs, nil
}

func possibleConflict(svcs *corev1.ServiceList) bool {
	for _, svc := range svcs.Items {
		ports := svc.Spec.Ports
		for _, port := range ports {
			if port.NodePort == dockerRegistryNodePort {
				return true
			}
		}
	}
	return false
}

func randomNodePort() int32 {
	number := rand.Int31n(maxNodePort - minNodePort)
	return minNodePort + number
}
