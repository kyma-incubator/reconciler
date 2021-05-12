package config

import (
	"fmt"
	"reflect"
	"time"
)

//convertTimestampToTime is converting the value of timestamp db-column to a Time instance
func convertTimestampToTime(value interface{}) (interface{}, error) {
	if reflect.TypeOf(value).Kind() == reflect.String {
		layout := "2006-02-01 15:04:05"
		return time.Parse(layout, value.(string))
	}
	if time, ok := value.(time.Time); ok {
		return time, nil
	}
	return nil, fmt.Errorf("Failed to convert value '%s' (kind: %s) for field 'Created' to Time struct",
		value, reflect.TypeOf(value).Kind())
}
