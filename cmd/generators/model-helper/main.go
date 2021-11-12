package main

import (
	"flag"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"path"
	"reflect"
	"text/template"

	"github.com/kyma-incubator/reconciler/pkg/keb"
)

var (
	inFilePath  string
	outFilePath string
	status      keb.Status
	helpersTpl  = `package {{ .PackageName }}

import "fmt"

func ToStatus(in string) (Status, error) {

	for _, status := range []Status{
		{{ range .Statuses }}{{ . }}, 
		{{ end -}}
	} {
		if in == string(status) {
			return status, nil
		}
	}
	return Status(""), fmt.Errorf("Given string is not Status: %s", in)
}`
)

func main() {
	flag.StringVar(&inFilePath, "i", "", "the input file containing generated model")
	flag.StringVar(&outFilePath, "o", "", "the output file the, where the helper methods will be generated to")
	flag.Parse()

	info := &types.Info{
		Uses: make(map[*ast.Ident]types.Object),
	}

	fileSet := token.NewFileSet()

	parseFile, err := parser.ParseFile(fileSet, inFilePath, nil, 0)
	if err != nil {
		log.Fatal(err)
	}

	conf := types.Config{Importer: importer.Default()}

	statusType := reflect.TypeOf(status)
	packagePath := statusType.PkgPath()
	packageName := path.Base(packagePath)

	if _, err := conf.Check(packageName, fileSet, []*ast.File{parseFile}, info); err != nil {
		log.Fatal(err)
	}

	statusNames := make([]string, 0)
	for _, d := range parseFile.Decls {
		switch decl := d.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if vSpec, ok := spec.(*ast.ValueSpec); ok {
					vSpecType := info.TypeOf(vSpec.Type)
					if vSpecType.String() != statusType.String() {
						continue
					}
					for _, nameSpec := range vSpec.Names {
						statusNames = append(statusNames, nameSpec.Name)
					}
				}
			}
		}
	}

	data := struct {
		PackageName string
		Statuses    []string
	}{
		PackageName: packageName,
		Statuses:    statusNames,
	}

	tpl, err := template.New("helpers").Parse(helpersTpl)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create(outFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err = tpl.Execute(f, data); err != nil {
		log.Fatal(err)
	}
}
