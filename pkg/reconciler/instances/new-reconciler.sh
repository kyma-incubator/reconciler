#!/bin/bash

readonly compName="$1"

if [ -z "$compName" ]; then
  echo "Please provide the name of the component as argument."
  echo ""
  echo "Example: $0 istio"
  echo ""
  exit 1
fi

if [ -d "./${compName}" ]; then
  echo ""
  echo "Package '${compName}' already exists: please choose a different name."
  echo ""
  exit 1
fi

if [ ! -d "./example" ]; then
  echo ""
  echo "Mandatory package 'example' is missing. Scaffolding a new component-reconciler is not possible."
  echo ""
  exit 1
fi

######################

echo "Creating new component-reconciler package '${compName}'"
cp -r ./example "./${compName}"

mv "./${compName}/example.go" "./${compName}/${compName}.go"

######################

echo "Adjusting init function"
for file in ./"$compName"/*.go; do
  sed -i '' "s/example/${compName}/g" "$file"
done

######################

echo "Updating component-reconciler loader"
import=""
for directory in */ ; do
    if [ -d "$directory" -a "$(basename "$directory")" != "example" ]; then
      import="${import}
      //import required to register component-reconciler '${compName}' in reconciler-registry
      _ \"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/$(basename "$directory")\""
    fi
done
if [ -n "$import" ]; then
  echo "// This file is generated: manual changes will be overwritten!!!

package instances

import(
${import}
)
" > loader.go
go fmt loader.go > /dev/null
fi

######################

echo ""
echo "Please edit ${compInitFile}: inject your reconcilication logic by setting your custom Action structs!"
echo ""

