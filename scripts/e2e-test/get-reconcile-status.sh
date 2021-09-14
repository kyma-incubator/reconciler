#!/usr/bin/env sh

# Description: Returns Kyma reconciliation status

## ---------------------------------------------------------------------------------------
## Execution steps
## ---------------------------------------------------------------------------------------
export RECONCILE_STATUS_URL=$(<status_url.txt)
status=$(curl -sL http://"$RECONCILE_STATUS_URL" | jq -r .status)
echo "$status"