#!/bin/bash

readonly minMajorVersion=1
readonly minMinorVersion=18

if [ -z "$(which kubectl)" ]; then
  echo 'kubectl command not found in $PATH: version check not possible'
  exit 0
fi

majorVersion="$(kubectl version --client -o json | jq -r .clientVersion.major)"
minorVersion="$(kubectl version --client -o json | jq -r .clientVersion.minor)"

if [ "$majorVersion" -lt "$minMajorVersion" -o "$minorVersion" -lt "$minMinorVersion" ]; then
  echo "Please upgrade 'kubectl' command to version >= ${minMajorVersion}.${minMinorVersion}"
  exit 1
fi