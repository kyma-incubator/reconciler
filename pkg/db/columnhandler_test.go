package db

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestColumnHandler(t *testing.T) {
	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()

	t.Run("Validate model", func(t *testing.T) {
		validate := func(t *testing.T, testStruct *validateMe, expectValid bool) {
			colHdr, err := NewColumnHandler(testStruct, NewTestConnection(t), testLogger)
			require.NoError(t, err)
			err = colHdr.Validate()
			if expectValid {
				require.NoError(t, err, "Validation has to be successful")
			} else {
				require.Error(t, err)
				require.True(t, IsInvalidEntityError(colHdr.Validate()), "Validation has to fail with InvalidEntityError")
			}
		}

		testStruct := &validateMe{}
		validate(t, testStruct, false)

		//invalid
		testStruct.I = 123
		validate(t, testStruct, false)

		//invalid
		testStruct.I64 = 123
		validate(t, testStruct, false)

		//invalid
		testStruct.F64 = 123.5
		validate(t, testStruct, false)

		//invalid
		testStruct.Slc = []string{} //empty slice is not sufficient for NotNull
		validate(t, testStruct, false)

		//invalid
		testStruct.Slc = []string{"test"}
		validate(t, testStruct, false)

		//valid
		testStruct.S = "now I'm valid"
		validate(t, testStruct, true)
	})

	//valid model
	testStruct := &MockDbEntity{
		Col1: "testString",
		Col2: true,
		Col3: 123456789,
	}
	colHdr, err := NewColumnHandler(testStruct, NewTestConnection(t), testLogger)
	require.NoError(t, err)
	require.NoError(t, colHdr.Validate())

	t.Run("Get values", func(t *testing.T) {
		require.Equal(t, 3, len(colHdr.columns))

		colValsAll, err := colHdr.ColumnValues(false)
		require.NoError(t, err)
		require.Len(t, colValsAll, 3)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Contains(t, colValsAll, "testString")
		require.Contains(t, colValsAll, true)

		colValsWriteable, err := colHdr.ColumnValues(true)
		require.NoError(t, err)
		require.Len(t, colValsWriteable, 2)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Contains(t, colValsWriteable, "testString")
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
		colValsAll, err := colHdr.ColumnValuesCsv(false)
		require.NoError(t, err)
		require.Len(t, splitAndTrimCsv(colValsAll), 3)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Contains(t, splitAndTrimCsv(colValsAll), "testString")
		require.Contains(t, splitAndTrimCsv(colValsAll), "true")

		colValsWriteable, err := colHdr.ColumnValuesCsv(true)
		require.NoError(t, err)
		require.Len(t, splitAndTrimCsv(colValsWriteable), 2)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Contains(t, splitAndTrimCsv(colValsWriteable), "testString")
	})

	t.Run("Get column values as placeholder CSV", func(t *testing.T) {
		colValsPlcHdrsAll, err := colHdr.ColumnValuesPlaceholderCsv(false)
		require.NoError(t, err)
		require.Equal(t, "$1, $2, $3", colValsPlcHdrsAll)

		colValsPlcHdrsWriteable, err := colHdr.ColumnValuesPlaceholderCsv(true)
		require.NoError(t, err)
		require.Equal(t, "$1, $2", colValsPlcHdrsWriteable)
	})

	t.Run("Get column entries as CSV", func(t *testing.T) {
		kvPairsAll, _, err := colHdr.ColumnEntriesCsv(false)
		require.NoError(t, err)
		require.Len(t, splitAndTrimCsv(kvPairsAll), 3)
		//don't test for encrypted value 'col_3' (enc-values are unique and cannot be reproduced/predicted)
		require.Contains(t, splitAndTrimCsv(kvPairsAll), "col_1=testString")
		require.Contains(t, splitAndTrimCsv(kvPairsAll), "col_2=true")

		kvPairsWriteable, _, err := colHdr.ColumnEntriesCsv(true)
		require.NoError(t, err)
		require.Len(t, splitAndTrimCsv(kvPairsWriteable), 2)
		//don't test for encrypted value 'col_3' (enc-values are unique and cannot be reproduced/predicted)
		require.Contains(t, splitAndTrimCsv(kvPairsWriteable), "col_1=testString")
	})

	t.Run("Get column entries as placeholder CSV", func(t *testing.T) {
		kvPairsPlcHdrsAll, plcHdrCnt, err := colHdr.ColumnEntriesPlaceholderCsv(false)
		require.NoError(t, err)
		require.Regexp(t, regexp.MustCompile(`((col_1|col_2|col_3)=\$[1-3](, )?){3}`), kvPairsPlcHdrsAll)
		require.Equal(t, 3, plcHdrCnt)

		kvPairsPlcHdrsWriteonly, plcHdrCnt, err := colHdr.ColumnEntriesPlaceholderCsv(true)
		require.NoError(t, err)
		require.Regexp(t, regexp.MustCompile(`((col_1|col_3)=\$[1-2](, )?){2}`), kvPairsPlcHdrsWriteonly)
		require.Equal(t, 2, plcHdrCnt)
	})
}

func splitAndTrimCsv(csv string) []string {
	var result []string
	for _, token := range strings.Split(csv, ",") {
		result = append(result, strings.TrimSpace(token))
	}
	return result
}

//mock DB entity used to test the validation logic
type validateMe struct {
	S   string   `db:"notNull"`
	B   bool     `db:"notNull"`
	I   int      `db:"notNull"`
	I64 int64    `db:"notNull"`
	F64 float64  `db:"notNull"`
	Slc []string `db:"notNull"`
}

func (fake *validateMe) String() string {
	return "I'm just used for testing the validation"
}

func (fake *validateMe) New() DatabaseEntity {
	return &validateMe{}
}

func (fake *validateMe) Table() string {
	return "validateMe"
}

func (fake *validateMe) Equal(_ DatabaseEntity) bool {
	return false
}

func (fake *validateMe) Marshaller() *EntityMarshaller {
	return NewEntityMarshaller(&fake)
}
