package db

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestColumnHandler(t *testing.T) {
	t.Run("Validate model", func(t *testing.T) {
		testStruct := &MockDbEntity{
			Col2: true,
			Col3: 123456789,
		}
		colHdr, err := NewColumnHandler(testStruct)
		require.NoError(t, err)
		require.True(t, IsInvalidEntityError(colHdr.Validate()))
	})

	//valid model
	testStruct := &MockDbEntity{
		Col1: "testString",
		Col2: true,
		Col3: 123456789,
	}
	colHdr, err := NewColumnHandler(testStruct)
	require.NoError(t, err)
	require.NoError(t, colHdr.Validate())

	t.Run("Get values", func(t *testing.T) {
		//check internal state
		require.Equal(t, 3, len(colHdr.columns))
		require.ElementsMatch(t, []interface{}{"testString", true, 123456789}, colHdr.ColumnValues(false))
		require.ElementsMatch(t, []interface{}{"testString", 123456789}, colHdr.ColumnValues(true))
	})

	t.Run("Get column name", func(t *testing.T) {
		colNameInt64, err := colHdr.ColumnName("Col1")
		require.NoError(t, err)
		require.Equal(t, "col_1", colNameInt64)
		colNameStr, err := colHdr.ColumnName("Col2")
		require.NoError(t, err)
		require.Equal(t, "col_2", colNameStr)
	})

	t.Run("Get column names as CSV", func(t *testing.T) {
		require.ElementsMatch(t, []string{"col_1", "col_2", "col_3"}, splitAndTrimCsv(colHdr.ColumnNamesCsv(false)))
		require.ElementsMatch(t, []string{"col_1", "col_3"}, splitAndTrimCsv(colHdr.ColumnNamesCsv(true)))
	})

	t.Run("Get column values as CSV", func(t *testing.T) {
		require.ElementsMatch(t, []string{"'testString'", "true", "123456789"}, splitAndTrimCsv(colHdr.ColumnValuesCsv(false)))
		require.ElementsMatch(t, []string{"'testString'", "123456789"}, splitAndTrimCsv(colHdr.ColumnValuesCsv(true)))
	})

	t.Run("Get column values as placeholder CSV", func(t *testing.T) {
		require.Equal(t, "$1, $2, $3", colHdr.ColumnValuesPlaceholderCsv(false))
		require.Equal(t, "$1, $2", colHdr.ColumnValuesPlaceholderCsv(true))
	})

	t.Run("Get column entries as CSV", func(t *testing.T) {
		require.ElementsMatch(t, []string{"col_1='testString'", "col_2=true", "col_3=123456789"}, splitAndTrimCsv(colHdr.ColumnEntriesCsv(false)))
		require.ElementsMatch(t, []string{"col_1='testString'", "col_3=123456789"}, splitAndTrimCsv(colHdr.ColumnEntriesCsv(true)))
	})

	t.Run("Get column entries as placeholder CSV", func(t *testing.T) {
		rwKeyValuePairsCsv := colHdr.ColumnEntriesPlaceholderCsv(false)
		require.Regexp(t, regexp.MustCompile(`((col_1|col_2|col_3)=\$[1-3](, )?){3}`), rwKeyValuePairsCsv)
		roKeyValuePairs := colHdr.ColumnEntriesPlaceholderCsv(true)
		require.Regexp(t, regexp.MustCompile(`((col_1|col_3)=\$[1-2](, )?){2}`), roKeyValuePairs)
	})
}

func splitAndTrimCsv(csv string) []string {
	result := []string{}
	for _, token := range strings.Split(csv, ",") {
		result = append(result, strings.TrimSpace(token))
	}
	return result
}
