package pod

import (
	"context"
	"testing"

	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"golang.org/x/sync/errgroup"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_NoActionHandler_ExecuteAndWaitFor(t *testing.T) {
	t.Run("should execute the NoActionHandler successfully", func(t *testing.T) {
		// given
		customObject := fixCustomObject()
		handler := NoActionHandler{}
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
