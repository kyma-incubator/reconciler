package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

var SupportedOutputFormats = []string{"table", "json", "json_pretty", "yaml"}

type OutputFormatter struct {
	header []string
	data   [][]interface{}
	format string
}

func NewOutputFormatter(format string) (*OutputFormatter, error) {
	for _, supportedFormat := range SupportedOutputFormats {
		if supportedFormat == format {
			return &OutputFormatter{
				format: format,
			}, nil
		}
	}
	return nil, fmt.Errorf("Output format '%s' is not supported: please choose between '%s'",
		format, strings.Join(SupportedOutputFormats, "', '"))
}

func (of *OutputFormatter) Header(header ...string) error {
	if of.data != nil {
		if err := of.headerColumnCheck(len(of.data), len(header)); err != nil {
			return err
		}
	}
	of.header = header
	return nil
}

func (of *OutputFormatter) AddRow(data ...interface{}) error {
	if of.header != nil {
		if err := of.headerColumnCheck(len(data), len(of.header)); err != nil {
			return err
		}
	}
	of.data = append(of.data, data)
	return nil
}

func (of *OutputFormatter) headerColumnCheck(columnCnt, headerCnt int) error {
	if columnCnt != headerCnt {
		return fmt.Errorf("Header count differs with column count: %d != %d", headerCnt, columnCnt)
	}
	return nil
}

func (of *OutputFormatter) Output(writer io.Writer) error {
	var err error
	switch of.format {
	case "table":
		err = of.tableOutput(writer)
	case "json_pretty":
		err = of.marshal(writer, func(data interface{}) ([]byte, error) {
			return json.MarshalIndent(data, "", "  ")
		})
	case "json":
		err = of.marshal(writer, json.Marshal)
	case "yaml":
		err = of.marshal(writer, yaml.Marshal)
	}
	return err
}

func (of *OutputFormatter) marshal(writer io.Writer, marshalFct func(interface{}) ([]byte, error)) error {
	data, err := of.serializeableData()
	if err == nil {
		json, err := marshalFct(data)
		if err != nil {
			return err
		}
		if _, err := writer.Write(json); err != nil {
			return err
		}
	}
	return nil
}

func (of *OutputFormatter) tableOutput(writer io.Writer) error {
	table := tablewriter.NewWriter(writer)
	table.SetHeader(of.header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	dataStr, err := of.dataAsStringSlice()
	if err != nil {
		return err
	}
	table.AppendBulk(dataStr)
	table.Render()
	return nil
}

func (of *OutputFormatter) dataAsStringSlice() ([][]string, error) {
	result := [][]string{}
	for _, dataRow := range of.data {
		resultRow := []string{}
		for _, dataField := range dataRow {
			var data string
			switch reflect.TypeOf(dataField).Kind() {
			case reflect.Slice:
				strSlice, ok := dataField.([]string)
				if ok {
					data = strings.Join(strSlice, ", ")
				} else {
					var buffer bytes.Buffer
					s := reflect.ValueOf(dataField)
					for i := 0; i < s.Len(); i++ {
						if buffer.Len() > 0 {
							buffer.WriteRune(',')
						}
						buffer.WriteString(fmt.Sprintf("%v", s.Index(i).Interface()))
					}
					data = buffer.String()
				}
			case reflect.Map:
				mapCasted, ok := dataField.(map[string]interface{})
				if ok {
					data = of.serializeMap(mapCasted)
				} else {
					mapSlicesCasted, ok := dataField.(map[string][]interface{})
					if !ok {
						return nil, fmt.Errorf("Cannot serialize data from map of type '%v'", dataField)
					}
					data = of.serializeMapWithSlices(mapSlicesCasted)
				}
			default:
				data = fmt.Sprintf("%v", dataField)
			}
			resultRow = append(resultRow, data)
		}
		result = append(result, resultRow)
	}
	return result, nil
}

func (of *OutputFormatter) serializeMap(data map[string]interface{}) string {
	keys := []string{}
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buffer bytes.Buffer
	for _, key := range keys {
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(fmt.Sprintf("%s=%v", key, data[key]))
	}
	return buffer.String()
}

func (of *OutputFormatter) serializeMapWithSlices(data map[string][]interface{}) string {
	keys := []string{}
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buffer bytes.Buffer
	for _, key := range keys {
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(fmt.Sprintf("%s: ", key))
		for _, payload := range data[key] {
			buffer.WriteString(fmt.Sprintf("%v", payload))
		}
	}
	return buffer.String()
}

func (of *OutputFormatter) serializeableData() ([]map[string]interface{}, error) {
	if len(of.header) == 0 {
		return nil, fmt.Errorf("No headers defined: cannot convert data to map")
	}
	data := []map[string]interface{}{}
	for _, dataRow := range of.data {
		dataTuple := make(map[string]interface{})
		for idxCol, hdr := range of.header {
			dataTuple[strcase.ToLowerCamel(hdr)] = dataRow[idxCol]
		}
		data = append(data, dataTuple)
	}
	return data, nil
}
