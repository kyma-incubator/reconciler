package pod

import (
	"context"
	"testing"

	"github.com/avast/retry-go"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_NoActionHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the NoActionHandler successfully", func(t *testing.T) {
		// given
		testObject := "test"
		testObjectNs := "testNs"
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: testObject, Namespace: testObjectNs},
		}
		customObject := CustomObject{
			Name:      testObject,
			Namespace: testObjectNs,
			Kind:      "Pod",
		}
		handler := NoActionHandler{struct {
			kubeClient kubernetes.Interface
			retryOpts  []retry.Option
			log        *zap.SugaredLogger
			debug      bool
			waitOpts   WaitOptions
		}{kubeClient: fake.NewSimpleClientset(&pod), retryOpts: []retry.Option{retry.Attempts(3)}, log: log.NewLogger(true), debug: true, waitOpts: WaitOptions{
			Interval: 10,
			Timeout:  30,
		}}}
		g := new(errgroup.Group)
		ctx := context.Background()
		defer ctx.Done()

		// when
		g.Go(func() error {
			err := handler.ExecuteAndWaitFor(ctx, customObject)
			if err != nil {
				return err
			}
			return nil
		})

		// then
		err := g.Wait()
		require.NoError(t, err)
	})
}
func Test_DeleteObjectHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the DeleteObjectHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := DeleteObjectHandler{handlerCfg{log: log.NewLogger(true), debug: true}}
		g := new(errgroup.Group)
		ctx := context.Background()
		defer ctx.Done()
		// when
		g.Go(func() error {
			err := handler.ExecuteAndWaitFor(ctx, *customObject)
			if err != nil {
				return err
			}
			return nil
		})

		// then
		err := g.Wait()
		require.NoError(t, err)
	})
}
func Test_RolloutHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the RolloutHandler successfully", func(t *testing.T) {
		// given
		pod := fixCustomObject()
		handler := RolloutHandler{handlerCfg{log: log.NewLogger(true), debug: true}}
		g := new(errgroup.Group)
		ctx := context.Background()
		defer ctx.Done()

		// when
		g.Go(func() error {
			err := handler.ExecuteAndWaitFor(ctx, *pod)
			if err != nil {
				return err
			}
			return nil
		})

		// then
		err := g.Wait()
		require.NoError(t, err)
	})
}
func Test_getParentObjectFromOwnerReferences(t *testing.T) {
	t.Run("should return empty parentObject when there is no owner references", func(t *testing.T) {
		// given
		ownerReference := []metav1.OwnerReference{}

		// when
		parentObjectData := getParentObjectFromOwnerReferences(ownerReference)

		// then
		require.NotNil(t, parentObjectData)
		require.Empty(t, parentObjectData)
	})

	t.Run("should return empty parentObject when owner reference is nil", func(t *testing.T) {
		// when
		parentObjectData := getParentObjectFromOwnerReferences(nil)

		// then
		require.NotNil(t, parentObjectData)
		require.Empty(t, parentObjectData)
	})

	t.Run("should return parentObject with data from one owner reference", func(t *testing.T) {
		// given
		ownerReference := []metav1.OwnerReference{{Name: "Name", Kind: "Kind"}}

		// when
		parentObjectData := getParentObjectFromOwnerReferences(ownerReference)

		// then
		require.NotNil(t, parentObjectData)
		require.Equal(t, parentObject{Name: "Name", Kind: "Kind"}, parentObjectData)
	})

	t.Run("should return parentObject with data from the first owner reference when there are two owner references", func(t *testing.T) {
		// given
		ownerReference := []metav1.OwnerReference{{Name: "Name1", Kind: "Kind1"}, {Name: "Name2", Kind: "Kind2"}}

		// when
		parentObjectData := getParentObjectFromOwnerReferences(ownerReference)

		// then
		require.NotNil(t, parentObjectData)
		require.Equal(t, parentObject{Name: "Name1", Kind: "Kind1"}, parentObjectData)
	})
}

func fixCustomObject() *CustomObject {
	return &CustomObject{
		Name:      "testObject",
		Namespace: "testNamespace",
		Kind:      "ReplicaSet",
	}
}
