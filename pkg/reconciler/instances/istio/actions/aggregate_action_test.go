package actions_test

import (
	"fmt"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/actions"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var errInner = errors.New("inner err")
var errOuter = fmt.Errorf(actions.AggregateErrorFormat, 1, &mockAction{runCalled: true, returnErr: true}, errInner)

var _ service.Action = &mockAction{}

type mockAction struct {
	runCalled bool
	returnErr bool
}

func (m *mockAction) Run(_ *service.ActionContext) error {
	m.runCalled = true
	if m.returnErr {
		return errInner
	}
	return nil
}

func TestActionAggregate(t *testing.T) {
	t.Run("should run all actions if none return error", func(t *testing.T) {
		// given
		m1 := mockAction{}
		m2 := mockAction{}
		m3 := mockAction{}

		aggregate := actions.NewActionAggregate(&m1, &m2, &m3)

		// when
		err := aggregate.Run(&service.ActionContext{})

		// then
		require.NoError(t, err)
		require.True(t, m1.runCalled)
		require.True(t, m2.runCalled)
		require.True(t, m3.runCalled)
	})

	t.Run("should return error if one action fails", func(t *testing.T) {
		// given
		m1 := mockAction{}
		m2 := mockAction{returnErr: true}
		m3 := mockAction{}
		aggregate := actions.NewActionAggregate(&m1, &m2, &m3)

		// when
		err := aggregate.Run(&service.ActionContext{})

		// then
		require.ErrorIs(t, err, errInner)
		require.Equal(t, err, errOuter)
		require.True(t, m1.runCalled)
		require.True(t, m2.runCalled)
		require.False(t, m3.runCalled)
	})
}
