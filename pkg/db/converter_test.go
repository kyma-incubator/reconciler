package db

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type simpleStruct struct {
	String string
	Bool   bool `db:"readOnly"`
	Int64  int64
}

func TestStructTableConverter(t *testing.T) {
	testStruct := simpleStruct{
		String: "testString",
		Bool:   true,
		Int64:  123456789,
	}
	stv, err := NewStructTableConverter(testStruct)
	require.NoError(t, err)

	//check internal state
	require.Equal(t, 3, len(stv.columns))

	//get values
	require.ElementsMatch(t, []interface{}{"testString", true, int64(123456789)}, stv.ColumnValues(false))
	require.ElementsMatch(t, []interface{}{"testString", int64(123456789)}, stv.ColumnValues(true))

	//get column names as CSV string
	require.ElementsMatch(t, []string{"string", "bool", "int_64"}, splitAndTrimCsv(stv.ColumnNamesCsv(false)))
	require.ElementsMatch(t, []string{"string", "int_64"}, splitAndTrimCsv(stv.ColumnNamesCsv(true)))

	//get column values as CSV string
	require.ElementsMatch(t, []string{"'testString'", "true", "123456789"}, splitAndTrimCsv(stv.ColumnValuesCsv(false)))
	require.ElementsMatch(t, []string{"'testString'", "123456789"}, splitAndTrimCsv(stv.ColumnValuesCsv(true)))

	//get column values as placeholder CSV string
	require.Equal(t, "$1, $2, $3", stv.ColumnValuesPlaceholderCsv(false))
	require.Equal(t, "$1, $2", stv.ColumnValuesPlaceholderCsv(true))

	//get column key-value pairs as CSV string
	require.ElementsMatch(t, []string{"string='testString'", "bool=true", "int_64=123456789"}, splitAndTrimCsv(stv.ColumnEntriesCsv(false)))
	require.ElementsMatch(t, []string{"string='testString'", "int_64=123456789"}, splitAndTrimCsv(stv.ColumnEntriesCsv(true)))

	//get column key-value pairs as placeholder CSV string
	rwKeyValuePairsCsv := stv.ColumnEntriesPlaceholderCsv(false)
	require.Regexp(t, regexp.MustCompile(`((string|bool|int_64)=\$[1-3](, )?){3}`), rwKeyValuePairsCsv)
	roKeyValuePairs := stv.ColumnEntriesPlaceholderCsv(true)
	require.Regexp(t, regexp.MustCompile(`((string|int_64)=\$[1-2](, )?){2}`), roKeyValuePairs)
}

func splitAndTrimCsv(csv string) []string {
	result := []string{}
	for _, token := range strings.Split(csv, ",") {
		result = append(result, strings.TrimSpace(token))
	}
	return result
}
