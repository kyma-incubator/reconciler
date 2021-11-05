# Reconciler

## Overview

The Reconciler is a central system to reconcile Kyma clusters.

## Run Reconciler locally (Mac OS)

Follow these steps to run Reconciler locally:

1. Build the Reconciler binary:

   ```bash
   make build-darwin 
   ```

2. Run Reconciler.
   > **CAUTION:** At the moment, reconciliation with default values will fail. You must specify additional parameters:
   >   ```bash
   >   ./bin/mothership-darwin local --value global.ingress.domainName=example.com,global.domainName=example.com
   >   ```
   
   We recommend specifying your own component list by using the `components` flag. By default, Reconciler installs all components listed in the [`components.yaml`](https://github.com/kyma-project/kyma/blob/main/installation/resources/components.yaml) file.
   ```bash
    ./bin/mothership-darwin local --components tracing,monitoring
   ```

## Testing

### Unit tests

To execute unit tests, use the `make test` target:

      make test


### Integration test

Integration tests have a higher resource consumption compared to unit tests. You must define the environment variable `KUBECONFIG` that points to a test Kubernetes cluster. 

Be aware that the integration test suite installs and deletes Kubernetes resources during the test run.

To execute the integration tests, execute the `make test-all` target:

     make test-all


## Adding a new component reconciler

If a custom logic must be executed before, during, or after the reconciliation of a component, component reconcilers are required.

The reconciler supports component reconcilers, which handle component-specific reconciliation runs.

To add another component reconciler, execute the following steps:

1. **Create a component reconciler** by executing the script `pkg/reconciler/instances/reconcilerctl.sh`.

   Provide the name of the component as parameter, for example:
   
       pkg/reconciler/instances/reconcilerctl.sh add istio

    The script creates a new package including the boilerplate code required to initialize a
    new component reconciler instance during runtime.

 2. **Edit the files inside the package**
   
     - Edit the file `action.go` and encapsulate your custom reconciliation logic in `Action` structs.

     - Edit the `$componentName.go` file:

       - Use the `WithDependencies()` method to list the components that are required before this reconciler can run.
       - Use the `WithPreReconcileAction()`, `WithReconcileAction()`, `WithPostReconcileAction()` to inject custom `Action` instances into the reconciliation process.

3. **Re-build the CLI** to add the new component reconciler to the `reconciler start` command.

   The `reconciler start` command is a convenient way to run a component reconciler as standalone server.

    Example:

        # Build CLI
        cd $GOPATH/src/github.com/kyma-incubator/reconciler/
        make build-darwin
        
        # Start the component reconciler (for example, 'istio') as standalone service
        ./bin/reconciler-darwin start istio
        
        # To get a list of all configuration options for the component reconciler, call: 
        ./bin/reconciler-darwin start istio --help

4. **Add component name to the list** in the Helm chart [`values.yaml`](https://github.com/kyma-project/control-plane/blob/main/resources/kcp/values.yaml#L53) and update the image version to the latest one after you merge your changes.
