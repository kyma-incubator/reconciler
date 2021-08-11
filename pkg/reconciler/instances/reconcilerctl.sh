#!/bin/bash

function showHelp() {
  echo "Provide all mandatory parameters:"
  echo ""
  echo "$0 [update | add [reconcilerName]]"
  echo ""
  echo "Example:"
  echo "$0 add istio   #adds a new component-reconciler with name 'istio'"
  echo "$0 update      #update component-reconciler loading mechanism"
  exit 1
}

function updateLoader() {
  echo "Updating component reconciler loader"
  local import=""
  for directory in */ ; do
      local baseName=$(basename "$directory")
      if [ -d "$directory" -a "$baseName" != "example" ]; then
        import="${import}
        //import required to register component reconciler '$baseName' in reconciler registry
        _ \"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/${baseName}\""
      fi
  done
  if [ -n "$import" ]; then
    echo "// This file is generated: manual changes will be overwritten!!!

  package instances

  import(${import}
  )
  " > loader.go
  go fmt loader.go > /dev/null
  fi
}

function addReconciler {
  local reconName="$1"
  local pkgName="${reconName//[.\-_,]}"

  if [ -d "./${pkgName}" ]; then
    echo ""
    echo "Package '${pkgName}' already exists. Choose a different name."
    echo ""
    exit 1
  fi

  if [ ! -d "./example" ]; then
    echo ""
    echo "Mandatory package 'example' is missing. Scaffolding a new component reconciler is not possible."
    echo ""
    exit 1
  fi

  echo "Creating new component reconciler package '${pkgName}'"
  cp -r ./example "./${pkgName}"

  mv "./${pkgName}/example.go" "./${pkgName}/${pkgName}.go"

  echo "Adjusting init function for component '${reconName}' (OS is '$(uname)')"
  for file in ./"${pkgName}"/*.go; do
    if [ "$(uname)" == "Darwin" ]; then
      sed -i '' "s/example/${reconName}/g" "$file"
    else
      sed -i "s/example/${reconName}/g" "$file"
    fi
  done

  echo ""
  echo "Edit '${pkgName}/*.go': Inject your reconciliation logic by setting your custom Action structs."
  echo ""
}


# Run command

cd $(dirname "$0")

readonly action="$1"

case "$action" in
   update)
     updateLoader
     ;;

   add)
    readonly reconcilerName="$2"
    if [ -z "$reconcilerName" ]; then
      showHelp
    fi
     addReconciler "$reconcilerName"
     updateLoader
     ;;

   *)
     showHelp
     ;;
esac
