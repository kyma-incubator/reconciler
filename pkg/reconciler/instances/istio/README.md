# Istio Reconciler

## Overview

Istio Reconciler manages the [Istio](https://github.com/kyma-project/kyma/tree/main/resources/istio) Kyma component. The component in Kyma defines [which version of the Istio module should be installed](https://github.com/kyma-project/kyma/blob/main/resources/istio/templates/istio-manager-config.yaml).

## Usage

Follow these steps to run Istio Reconciler on your local machine:

1. Export the **KUBECONFIG** variable pointing to your cluster.

   ```bash
   export KUBECONFIG={PATH_TO_THE_KUBECONFIG_FILE}
   ```

2. Build the Reconciler binary:

   ```bash
   make build-darwin
   ```

3. Run Istio Reconciler:

   ```bash
    ./bin/mothership-darwin local --components istio
   ```

## Details

Reconciliation in Kyma is handled by Reconciler. The Mothership Reconciler knows the reconciliation status of every managed Kyma cluster and initiates reconciliation of all Kyma components.

The Istio component's reconciler is responsible for installing and upgrading the [Istio Module](https://github.com/kyma-project/istio) on the cluster. It also ensures that the Istio custom resource remains in the `Ready` state.

## Istio reconciler workflow

1. Istio reconciler fetches the version of the Istio module that needs to be installed from the Kyma charts definition located in `resources/istio/templates/istio-manager-config.yaml`.
2. The Istio module's installation manifests and the default Istio CR are fetched from the [Istio module release](https://github.com/kyma-project/istio/releases).
3. The module's installation charts are either installed or updated.
4. If there is no Istio CR present, the reconciler creates a new one. This triggers the reconciliation of `istio-manager`.
5. The reconciler waits until the Istio module is in the `Ready` state and then finishes the reconciliation process.
