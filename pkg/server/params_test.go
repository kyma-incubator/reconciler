package server

import (
	"bytes"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

const (
	fakeURL = "https://host.com/my/dummy/url?strSlice=abc&strSlice=xyz&int=123&int64=123&string=string"
)

func TestParams(t *testing.T) {

	t.Run("Without router", func(t *testing.T) {
		req := newRequest(t)

		params := NewParams(req)

		strSlice, err := params.StrSlice("strSlice")
		require.NoError(t, err)
		require.Equal(t, []string{"abc", "xyz"}, strSlice)

		str, err := params.String("string")
		require.NoError(t, err)
		require.Equal(t, "string", str)

		i, err := params.Int("int")
		require.NoError(t, err)
		require.Equal(t, 123, i)

		i64, err := params.Int64("int64")
		require.NoError(t, err)
		require.Equal(t, int64(123), i64)
	})

	t.Run("With router", func(t *testing.T) {
		req := newRequest(t)

		//configure MUX URL-vars
		req = mux.SetURLVars(req, map[string]string{
			"string": "stringURL",
			"int":    "987",
			"int64":  "987",
		})

		params := NewParams(req)

		strSlice, err := params.StrSlice("strSlice")
		require.NoError(t, err)
		require.Equal(t, []string{"abc", "xyz"}, strSlice)

		str, err := params.String("string")
		require.NoError(t, err)
		require.Equal(t, "stringURL", str)

		i, err := params.Int("int")
		require.NoError(t, err)
		require.Equal(t, 987, i)

		i64, err := params.Int64("int64")
		require.NoError(t, err)
		require.Equal(t, int64(987), i64)
	})
}

func newRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest(http.MethodGet, fakeURL, bytes.NewBuffer([]byte{}))
	require.NoError(t, err)
	return req
}
