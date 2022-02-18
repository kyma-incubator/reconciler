package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func ReadFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	return data
}

func ReadManifest(t *testing.T, fileName string) string {
	return string(ReadFile(t, filepath.Join("test", fileName)))
}

func ReadManifestToUnstructured(t *testing.T, filename string) []*unstructured.Unstructured {
	manifest := ReadManifest(t, filename)

	var result []*unstructured.Unstructured
	for _, resourceYAML := range strings.Split(string(manifest), "---") {
		if strings.TrimSpace(resourceYAML) == "" {
			continue
		}
		obj := &unstructured.Unstructured{}
		dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, _, err := dec.Decode([]byte(resourceYAML), nil, obj)
		require.NoError(t, err)

		result = append(result, obj)
	}

	return result
}
