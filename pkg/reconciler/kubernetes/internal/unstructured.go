package internal

import (
	"bufio"
	"bytes"
	"io"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	yamlToJson "sigs.k8s.io/yaml"
)

func ToUnstructured(manifest []byte, async bool) ([]*unstructured.Unstructured, error) {
	var result []*unstructured.Unstructured
	var err error

	chanMes, chanErr := readYaml(manifest, async)
Loop:
	for {
		select {
		case yamlData, ok := <-chanMes:
			if !ok {
				//channel closed
				break Loop
			}

			//convert YAML to JSON
			jsonData, err := yamlToJson.YAMLToJSON(yamlData)
			if err != nil {
				break Loop
			}

			if string(jsonData) == "null" {
				//YAML didn't contain any valuable JSON data (e.g. just comments)
				continue
			}

			//get unstructured entity from JSON and intercept
			unstruct, err := newUnstructured(jsonData)
			if err != nil {
				break Loop
			}
			result = append(result, unstruct)

		case err = <-chanErr:
			break Loop
		}
	}
	return result, err
}

func readYaml(data []byte, async bool) (<-chan []byte, <-chan error) {
	var (
		chanErr        = make(chan error)
		chanBytes      = make(chan []byte)
		multidocReader = utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	)

	readFct := func() {
		defer close(chanErr)
		defer close(chanBytes)

		for {
			buf, err := multidocReader.Read()
			if err != nil {
				if err == io.EOF {
					return
				}
				chanErr <- errors.Wrap(err, "failed to read yaml data")
				return
			}
			chanBytes <- buf
		}
	}

	if async {
		go readFct()
	} else {
		readFct()
	}

	return chanBytes, chanErr
}

//newUnstructured converts a map[string]interface{} to a kubernetes unstructured.Unstructured
//object.
//From https://github.com/billiford/go-clouddriver/blob/master/pkg/kubernetes/unstructured.go
func newUnstructured(b []byte) (*unstructured.Unstructured, error) {
	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(b, nil, nil)
	if err != nil {
		return nil, err
	}
	// Convert the runtime.Object to unstructured.Unstructured.
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{
		Object: m,
	}, nil
}
