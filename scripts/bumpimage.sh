#!/bin/bash

if [ -z "$PULL_NUMBER" ]; then
  >&2 echo "Environment variable 'PULL_NUMBER' is missing: cann update tag for PR-image"
  exit 1
fi

backupFlag=""
if [ "$(uname -s)" == 'Darwin' ]; then
  backupFlag="-I '.bak'"
fi

readonly valuesYaml="$(dirname $0)/../resources/reconciler/values.yaml"

sed $backupFlag -E "/tag:/ s/PR-[[:digit:]]+/PR-${PULL_NUMBER}/" $valuesYaml
readonly bumpExitCode=$?

if [ "$bumpExitCode" -eq "0" ]; then
  echo "New PR-image tag is: PR-${PULL_NUMBER}"
else
  exit $bumpExitCode
fi