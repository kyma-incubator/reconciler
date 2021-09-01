package chart

import (
	"bytes"
	"fmt"
)

type ManifestType string

const (
	CRD       ManifestType = "crd"
	HelmChart ManifestType = "helmChart"
)

type Manifest struct {
	Type     ManifestType
	Name     string
	Manifest string
}

func MergeManifests(manifests ...*Manifest) string {
	var buffer bytes.Buffer
	for _, manifest := range manifests {
		buffer.WriteString("---\n")
		buffer.WriteString(fmt.Sprintf("# Manifest of %s '%s'\n", manifest.Type, manifest.Name))
		buffer.WriteString(manifest.Manifest)
		buffer.WriteString("\n")
	}
	return buffer.String()
}
