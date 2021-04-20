package db

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type simpleStruct struct {
	String string
	Bool   bool
	Int64  int64
}

var (
	expectedColNames  = []string{"string", "bool", "int_64"}
	expectedColValues = []interface{}{"testString", true, int64(123456789)}
)

func TestStructTableConverter(t *testing.T) {
	testStruct := simpleStruct{
		String: "testString",
		Bool:   true,
		Int64:  123456789,
	}
	stv, err := NewStructTableConverter(testStruct)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedColNames, stv.columns)
	require.ElementsMatch(t, expectedColValues, stv.values)
	require.ElementsMatch(t, expectedColNames, splitAndTrimCsv(stv.ColumnNamesCsv()))
	require.ElementsMatch(t, []string{"'testString'", "true", "123456789"}, splitAndTrimCsv(stv.ColumnValuesCsv()))
	require.ElementsMatch(t, []string{"string='testString'", "bool=true", "int_64=123456789"}, splitAndTrimCsv(stv.ColumnEntriesCsv()))
}

func splitAndTrimCsv(csv string) []string {
	result := []string{}
	for _, token := range strings.Split(csv, ",") {
		result = append(result, strings.TrimSpace(token))
	}
	return result
}
