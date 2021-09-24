#!/usr/bin/env sh

# Description: Initiates Kyma reconciliation requests to reconciler and waits until Kyma is installed

## ---------------------------------------------------------------------------------------
## Configurations and Variables
## ---------------------------------------------------------------------------------------
set -e

readonly RECONCILER_HOST="http://reconciler-mothership-reconciler.reconciler"
readonly RECONCILER_TIMEOUT=1200 # in secs
readonly RECONCILER_DELAY=2 # in secs

readonly RECONCILE_API="${RECONCILER_HOST}/v1/clusters"
readonly RECONCILE_PAYLOAD_FILE="/tmp/body.json"

## ---------------------------------------------------------------------------------------
## Functions
## ---------------------------------------------------------------------------------------

# Waits until Kyma reconciliation is in ready state
function wait_until_kyma_installed() {
  iterationsLeft=$(( RECONCILER_TIMEOUT/RECONCILER_DELAY ))
  while : ; do
    status=$(curl -sL http://"$RECONCILE_STATUS_URL" | jq -r .status)
    if [ "${status}" = "ready" ]; then
      echo "Kyma is installed"
      exit 0
    fi

    if [ "$RECONCILER_TIMEOUT" -ne 0 ] && [ "$iterationsLeft" -le 0 ]; then
      echo "timeout reached on Kyma installation error. Exiting"
      exit 1
    fi

    sleep $RECONCILER_DELAY
    echo "waiting to get Kyma installed, current status: ${status} ...."
    iterationsLeft=$(( iterationsLeft-1 ))
  done
}

# Sends HTTP POST request to mothership-reconciler to trigger reconciliation of Kyma
function send_reconciliation_request() {
  echo "sending reconciliation request to mothership-reconciler at: ${RECONCILE_API}"
  statusURL=$(curl --request POST -sL \
       --url "${RECONCILE_API}"\
       --data @"${RECONCILE_PAYLOAD_FILE}" | jq -r .statusURL)

  statusURL=$(sed "s/mothership-reconciler/mothership-reconciler.reconciler/" <<< "${statusURL}")
  export RECONCILE_STATUS_URL="${statusURL}"
}

# Checks if the reconciler returned status url is valid or not
function check_reconcile_status_url() {
  echo "RECONCILE_STATUS_URL: ${RECONCILE_STATUS_URL}"
  if [[ ! $RECONCILE_STATUS_URL ]] || [[ "$RECONCILE_STATUS_URL" == "null" ]]; then
    echo "reconciliation request failed: RECONCILE_STATUS_URL is invalid"
    exit 1
  fi
}

## ---------------------------------------------------------------------------------------
## Execution steps
## ---------------------------------------------------------------------------------------

# Install curl and jq
echo "Installing curl and jq to the environment"
apk --no-cache add curl jq

# Send reconciliation http request to mothership-reconciler
send_reconciliation_request

# Check if reconcile status url is valid
check_reconcile_status_url

# Wait until Kyma is installed
wait_until_kyma_installed

echo "reconcile-kyma completed"