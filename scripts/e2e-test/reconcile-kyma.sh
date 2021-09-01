#!/usr/bin/env sh
set -e
## Add curl and jq
apk --no-cache add curl jq

## Execute HTTP PUT to mothership-reconciler to http://mothership-reconciler-host:port/v1/clusters
statusURL=$(curl --request POST -sL \
     --url 'http://reconciler-mothership-reconciler.reconciler/v1/clusters'\
     --data @/tmp/body.json | jq -r .statusUrl)
echo "statusURL: ${statusURL}"

if [[ ! $statusURL ]]; then
  echo "reconciliation request failed: statusURL is empty"
  exit 1
fi

## Wait until Kyma is installed
timeout=1200 # in secs
delay=2 # in secs
iterationsLeft=$(( timeout/delay ))
while : ; do
  status=$(curl -sL http://$statusURL | jq -r .status)
  if [ "${status}" = "ready" ]; then
    echo "kyma is installed"
    exit 0
  fi

  if [ "$timeout" -ne 0 ] && [ "$iterationsLeft" -le 0 ]; then
    echo "timeout reached on kyma installation error. Exiting"
    exit 1
  fi

  sleep $delay
  echo "waiting to get Kyma installed, current status: ${status} ...."
  iterationsLeft=$(( iterationsLeft-1 ))
done