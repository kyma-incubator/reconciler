package progress

import (
	"context"
	"sort"

	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
)

type ReplicaSetsByCreationTimestamp []*apps.ReplicaSet

func (o ReplicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o ReplicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o ReplicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

func GetNewReplicaSet(deployment *apps.Deployment, c appsclient.AppsV1Interface) (*apps.ReplicaSet, error) {

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: selector.String()}

	all, err := c.ReplicaSets(deployment.Namespace).List(context.TODO(), options)
	if err != nil {
		return nil, err
	}
	var replicaSets []*apps.ReplicaSet
	for i := range all.Items {
		if metav1.IsControlledBy(&all.Items[i], deployment) {
			replicaSets = append(replicaSets, &all.Items[i])
		}
	}

	if len(replicaSets) == 0 {
		return nil, nil
	}

	sort.Sort(ReplicaSetsByCreationTimestamp(replicaSets))

	return replicaSets[len(replicaSets) - 1], nil
}