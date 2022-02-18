# Istio Reconciler

## Overview

Istio Reconciler manages the [Istio](https://github.com/kyma-project/kyma/tree/main/resources/istio-configuration) Kyma component. We support two latest minor Kyma 2.x releases and the `main` Kyma version.

## Prerequisites

The Istio component requires `cluster-essentials` to be installed as a prerequisite.

## Usage

Follow these steps to run Istio Reconciler on your local machine:

1. Export the **KUBECONFIG** variable pointing to your cluster and the **ISTIOCTL_PATH** variable:

   ```bash
   export KUBECONFIG={PATH_TO_THE_KUBECONFIG_FILE}
   export ISTIOCTL_PATH={PATH_TO_THE_ISTIOCTL_BINARY}
   ```

2. Build the Reconciler binary:

   ```bash
   make build-darwin
   ```

3. Pass an appropriate domain name for the two values listed in the command, and run Istio Reconciler:

   ```bash
    ./bin/mothership-darwin local --value global.ingress.domainName=example.com,global.domainName=example.com --components cluster-essentials,istio
   ```
