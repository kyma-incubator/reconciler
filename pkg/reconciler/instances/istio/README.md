# istio-configuration reconciler

## Overview

The istio-configuration reconciler manages `istio-configuration` Kyma component.

## Run Reconciler locally (Mac OS)

Follow these steps to run istio-configuration reconciler locally on your cluster:

1. Export `KUBECONFIG` variable to point to your cluster.

   ```bash
   export KUBECONFIG=<path-to-the-kubeconfig-file>
   ```

2. Build the Reconciler binary:

   ```bash
   make build-darwin
   ```

3. Run Reconciler.
   > **CAUTION:** `istio-configuration` requires `cluster-essentials` component to be installed on the cluster
   > beforehand for Istio to be functioning properly. Also, pass appropriate domain name for the two values
   > listed in the command.

   ```bash
    ./bin/mothership-darwin local --value global.ingress.domainName=example.com,global.domainName=example.com --components cluster-essentials,istio-configuration
   ```
