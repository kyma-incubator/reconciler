package db

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type simpleStruct struct {
	String string `db:"notNull"`
	Bool   bool   `db:"readOnly"`
	Int64  int64
}

func TestColumnHandler(t *testing.T) {
	t.Run("Validate model", func(t *testing.T) {
		testStruct := simpleStruct{
			Bool:  true,
			Int64: 123456789,
		}
		stc, err := NewColumnHandler(testStruct)
		require.NoError(t, err)
		require.True(t, IsIncompleteEntityError(stc.Validate()))
	})

	//valid model
	testStruct := simpleStruct{
		String: "testString",
		Bool:   true,
		Int64:  123456789,
	}
	stc, err := NewColumnHandler(testStruct)
	require.NoError(t, err)
	require.NoError(t, stc.Validate())

	t.Run("Get values", func(t *testing.T) {
		//check internal state
		require.Equal(t, 3, len(stc.columns))
		require.ElementsMatch(t, []interface{}{"testString", true, int64(123456789)}, stc.ColumnValues(false))
		require.ElementsMatch(t, []interface{}{"testString", int64(123456789)}, stc.ColumnValues(true))
	})

	t.Run("Get column names as CSV", func(t *testing.T) {
		require.ElementsMatch(t, []string{"string", "bool", "int_64"}, splitAndTrimCsv(stc.ColumnNamesCsv(false)))
		require.ElementsMatch(t, []string{"string", "int_64"}, splitAndTrimCsv(stc.ColumnNamesCsv(true)))
	})

	t.Run("Get column values as CSV", func(t *testing.T) {
		require.ElementsMatch(t, []string{"'testString'", "true", "123456789"}, splitAndTrimCsv(stc.ColumnValuesCsv(false)))
		require.ElementsMatch(t, []string{"'testString'", "123456789"}, splitAndTrimCsv(stc.ColumnValuesCsv(true)))
	})

	t.Run("Get column values as placeholder CSV", func(t *testing.T) {
		require.Equal(t, "$1, $2, $3", stc.ColumnValuesPlaceholderCsv(false))
		require.Equal(t, "$1, $2", stc.ColumnValuesPlaceholderCsv(true))
	})
	t.Run("Get column entries as CSV", func(t *testing.T) {
		require.ElementsMatch(t, []string{"string='testString'", "bool=true", "int_64=123456789"}, splitAndTrimCsv(stc.ColumnEntriesCsv(false)))
		require.ElementsMatch(t, []string{"string='testString'", "int_64=123456789"}, splitAndTrimCsv(stc.ColumnEntriesCsv(true)))
	})

	t.Run("Get column entries as placeholder CSV", func(t *testing.T) {
		rwKeyValuePairsCsv := stc.ColumnEntriesPlaceholderCsv(false)
		require.Regexp(t, regexp.MustCompile(`((string|bool|int_64)=\$[1-3](, )?){3}`), rwKeyValuePairsCsv)
		roKeyValuePairs := stc.ColumnEntriesPlaceholderCsv(true)
		require.Regexp(t, regexp.MustCompile(`((string|int_64)=\$[1-2](, )?){2}`), roKeyValuePairs)
	})
}

func splitAndTrimCsv(csv string) []string {
	result := []string{}
	for _, token := range strings.Split(csv, ",") {
		result = append(result, strings.TrimSpace(token))
	}
	return result
}
