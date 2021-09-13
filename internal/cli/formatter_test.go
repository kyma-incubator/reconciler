package cli

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatter(t *testing.T) {
	for _, format := range SupportedOutputFormats {
		of, err := NewOutputFormatter(format)
		require.NoError(t, err)
		expected, err := ioutil.ReadFile(path.Join("test", "formatter", fmt.Sprintf("%s.txt", format)))
		require.NoError(t, err)
		require.Equal(t, string(expected), render(t, of))
	}
}

func render(t *testing.T, of *OutputFormatter) string {
	var err error
	err = of.Header("C1", "c2", "C3") //will be converted to lower-case for JSON/YAML format or upper-case for TABLE format
	require.NoError(t, err)

	err = of.AddRow("1.1", map[string]interface{}{"key2.1a": "val2.1a", "key2.1b": 3}, []string{"3.1a", "3.1b", "3.1c"})
	require.NoError(t, err)
	err = of.AddRow("1.2", map[string]interface{}{"key2.2a": true, "key2.2b": []string{"test"}}, []int{9, 8, 7})
	require.NoError(t, err)

	var buffer bytes.Buffer
	err = of.Output(&buffer)
	require.NoError(t, err)

	return buffer.String()
}
