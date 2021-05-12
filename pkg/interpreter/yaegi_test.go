package interpreter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYaegi(t *testing.T) {

	t.Run("Happy path with string result", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
"xyz"
`)
		result, err := goInt.EvalString()
		require.NoError(t, err)
		require.Equal(t, "xyz", result)
	})

	t.Run("Happy path with boolean as result", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
import "fmt"
fmt.Sprint("true")
`)
		result, err := goInt.EvalBool()
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("Happy path with boolean 'true' as result", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
var x = 1
5 > x
`)
		result, err := goInt.EvalBool()
		require.NoError(t, err)
		require.True(t, result)
	})

	t.Run("Happy path with boolean 'false' as result", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
var x = 1
5 < x
`)
		result, err := goInt.EvalBool()
		require.NoError(t, err)
		require.False(t, result)
	})

	t.Run("Happy path with string result and bindings", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
import "fmt"
fmt.Sprintf("foo=%s | x=%d | y=%t", foo, x, y)
`).WithBindings(map[string]interface{}{"foo": "bar", "x": 123, "y": true})
		result, err := goInt.EvalString()
		require.NoError(t, err)
		require.Equal(t, "foo=bar | x=123 | y=true", result)
	})

	t.Run("Invalid boolean result", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
"xyz"
`)
		_, err := goInt.EvalBool()
		require.Error(t, err)
		require.True(t, IsNoBooleanResultError(err))
	})

	t.Run("Invalid binding", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
import "fmt"
fmt.Sprintf("x=%v | y=%v", x, y)
`).WithBindings(map[string]interface{}{"x": map[string]string{"test": "shouldFail"}, "y": []string{"fail", "test"}})
		_, err := goInt.EvalString()
		require.Error(t, err)
		require.True(t, strings.HasPrefix(err.Error(), "Cannot bind key"))
	})

	t.Run("Block denied import (one line)", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
import "os"
os.GetEnv("ABC")
`)
		_, err := goInt.Eval()
		require.Error(t, err)
		require.True(t, IsBlockedImportError(err))
	})

	t.Run("Block denied import (multi line)", func(t *testing.T) {
		goInt := NewGolangInterpreter(`
import (
	"os"
	"net"
)
os.GetEnv("ABC")
`)
		_, err := goInt.Eval()
		require.Error(t, err)
		require.True(t, IsBlockedImportError(err))
	})

}
