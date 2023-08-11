# Istio Reconciler

## Overview

Istio Reconciler manages the [Istio](https://github.com/kyma-project/kyma/tree/main/resources/istio) Kyma component. The component in Kyma provides definition of [which version of Istio module should be installed](https://github.com/kyma-project/kyma/blob/main/resources/istio/templates/istio-manager-config.yaml).

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

Istio component reconciler resposibility is installation and upgrades of [Istio Module](https://github.com/kyma-project/istio) on the cluster and keeping the state of Istio Custom Resource Ready.

## Istio reconciler workflow

1. Istio reconciler fetches the version of to-be-installed istio module from Kyma charts definition under `resources/istio/templates/istio-manager-config.yaml`
2. Istio module installation manifests and default Istio CR is fetched from [Istio module release](https://github.com/kyma-project/istio/releases)
3. The module installation charts are installed or updated
4. If there is no Istio Custom Resource present, reconciler creates a new one, triggering istio-manager reconcilation
5. Reconciler wait's until Istio module is Ready, then finishes reconciling
