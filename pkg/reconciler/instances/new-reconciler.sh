#!/bin/bash

readonly compName="$1"

if [ "$compName" = "" ]; then
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

echo "Creating new component-reconciler package"
cp -r ./example "./${compName}"

mv "./${compName}/example.go" "./${compName}/${compName}.go"

echo "Adjusting component-reconciler init function"
for file in ./"$compName"/*.go; do
  sed -i '' "s/example/${compName}/g" "$file"
done

echo ""
echo "Please edit ${compInitFile}: inject your reconcilication logic by setting your custom Action structs!"
echo ""