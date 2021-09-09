package connectivityproxy

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/stretchr/testify/require"
)

func TestAction(t *testing.T) {
	t.Run("Should invoke operations", func(t *testing.T) {
		invoked := 0
		action := CustomAction{
			name: "testAction",
			copyFactory: []CopyFactory{
				func(context *service.ActionContext) *SecretCopy {
					invoked++
					return nil
				},
				func(context *service.ActionContext) *SecretCopy {
					invoked++
					return nil
				},
			},
		}

		err := action.Run("", "", nil, nil)

		require.NoError(t, err)
		require.Equal(t, 2, invoked)
	})
}
