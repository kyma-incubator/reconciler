package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntityMarshaller(t *testing.T) {
	mock := &MockDbEntity{
		Col1: "foo",
		Col2: true,
		Col3: 123,
	}

	t.Run("Test marshalling - happy path", func(t *testing.T) {
		marshaller := NewEntityMarshaller(mock)
		marshaller.AddMarshaller("Col1", func(value interface{}) (interface{}, error) {
			require.Equal(t, "foo", value)
			return "bar", nil
		})
		marshaller.AddMarshaller("Col3", func(value interface{}) (interface{}, error) {
			require.Equal(t, 123, value)
			return 987, nil
		})
		data, err := marshaller.Marshal()
		require.NoError(t, err)
		require.Equal(t, map[string]interface{}{"Col1": "bar", "Col2": true, "Col3": 987}, data)
	})

	t.Run("Test marshalling with failure", func(t *testing.T) {
		marshaller := NewEntityMarshaller(mock)
		marshaller.AddMarshaller("Col1", func(value interface{}) (interface{}, error) {
			return nil, fmt.Errorf("I don't like the value")
		})
		_, err := marshaller.Marshal()
		require.EqualError(t, err, "I don't like the value")
	})

	t.Run("Test unmarshalling - happy path", func(t *testing.T) {
		unmarshalledMock := &MockDbEntity{}
		marshaller := NewEntityMarshaller(unmarshalledMock)
		marshaller.AddUnmarshaller("Col1", func(value interface{}) (interface{}, error) {
			require.Equal(t, "bar", value)
			return "foo", nil
		})
		marshaller.AddUnmarshaller("Col2", func(value interface{}) (interface{}, error) {
			require.Equal(t, false, value)
			return true, nil
		})
		err := marshaller.Unmarshal(map[string]interface{}{"Col1": "bar", "Col2": false, "Col3": 123})
		require.NoError(t, err)
		require.Equal(t, mock, unmarshalledMock)
	})

	t.Run("Test unmarshalling with failure", func(t *testing.T) {
		marshaller := NewEntityMarshaller(&MockDbEntity{})
		marshaller.AddUnmarshaller("Col1", func(value interface{}) (interface{}, error) {
			return nil, fmt.Errorf("I don't like the value")
		})
		err := marshaller.Unmarshal(map[string]interface{}{"Col1": "bar", "Col2": false, "Col3": 123})
		require.EqualError(t, err, "I don't like the value")
	})
}
