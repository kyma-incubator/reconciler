package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

var SupportedOutputFormats = []string{"table", "json", "yaml"}

type OutputFormatter struct {
	header []string
	data   [][]string
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

func (of *OutputFormatter) AddRow(data ...string) error {
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
		of.tableOutput(writer)
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

func (of *OutputFormatter) tableOutput(writer io.Writer) {
	table := tablewriter.NewWriter(writer)
	table.SetHeader(of.header)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(of.data)
	table.Render()
}

func (of *OutputFormatter) serializeableData() ([]map[string]string, error) {
	if len(of.header) == 0 {
		return nil, fmt.Errorf("No headers defined: cannot convert data to map")
	}
	data := []map[string]string{}
	for _, dataRow := range of.data {
		dataTuple := make(map[string]string)
		for idxCol, hdr := range of.header {
			dataTuple[hdr] = dataRow[idxCol]
		}
		data = append(data, dataTuple)
	}
	return data, nil
}
