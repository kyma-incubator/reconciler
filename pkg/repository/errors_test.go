package repository_test

import (
	"github.com/kyma-incubator/reconciler/pkg/repository"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestIsNotFoundErr(t *testing.T) {
	t.Run("Success - found not found err", func(t *testing.T) {
		//GIVEN
		r, err := repository.NewRepository(nil, false)
		require.NoError(t, err)
		err = errors.New("basic error")
		err = errors.Wrap(err, "wrapping")
		err = r.NewNotFoundError(err, nil, nil)
		err = errors.Wrap(err, "wrapping not found")

		//WHEN
		out := repository.IsNotFoundError(err)

		//THEN
		require.True(t, out)
	})
	t.Run("Success - no not found err", func(t *testing.T) {
		//GIVEN
		err := errors.New("basic error")
		err = errors.Wrap(err, "wrapping")
		err = errors.Wrap(err, "wrapping not found")

		//WHEN
		out := repository.IsNotFoundError(err)

		//THEN
		require.False(t, out)
	})
}
