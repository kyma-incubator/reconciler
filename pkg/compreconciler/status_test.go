package compreconciler

import (
	"context"
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type statusUpdate struct {
	time   time.Time
	status Status
}

func TestStatusUpdater(t *testing.T) {
	if !test.RunExpensiveTests() {
		return
	}

	t.Parallel()

	t.Run("Test status updater without timeout", func(t *testing.T) {
		ctx, _ := context.WithCancel(context.Background())

		callMe := make(chan Status, 10)
		defer close(callMe)

		lch, err := newLocalCallbackHandler(func(status Status) error {
			callMe <- status
			return nil
		}, true)
		require.NoError(t, err)

		statusUpdater := newStatusUpdater(ctx, 1*time.Second, lch, uint(1), true)

		require.NoError(t, statusUpdater.Start())
		time.Sleep(2 * time.Second)

		statusUpdater.Failed()
		time.Sleep(2 * time.Second)

		statusUpdater.Success()
		time.Sleep(2 * time.Second)

		for status := range callMe {
			fmt.Println(status)
		}
	})

	t.Run("Test status updater with timeout", func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)

		lch, err := newLocalCallbackHandler(func(status Status) error {
			return nil
		}, true)
		require.NoError(t, err)

		statusUpdater := newStatusUpdater(ctx, 1*time.Second, lch, uint(1), true)

		require.NoError(t, statusUpdater.Start())
		time.Sleep(4 * time.Second) //wait longer than timeout to force closed context
	})
}
