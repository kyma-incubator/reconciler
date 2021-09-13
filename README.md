# Reconciler

## Overview

>**CAUTION:** This repository is in a very early stage. Use it at your own risk.

The Reconciler is a central system to reconcile Kyma clusters.

## Run Reconciler locally (Mac OS)

Follow these steps to run Reconciler locally:

1. Build the Reconciler binary:

```
make build-darwin 
```


2. Run Reconciler with the default configuration. By default, Reconciler installs all components listed in the [components.yaml](https://github.com/kyma-project/kyma/blob/main/installation/resources/components.yaml) file.

```
./bin/reconciler-darwin local 
```

You can also run Reconciler with the specified components:
```
./bin/reconciler-darwin local --components tracing,monitoring --value tracing.key=value,global.key=value
```

## Testing

### Unit tests

Use the `make test` target to execute unit tests:

      make test


### Integration test

Integration tests have a higher resource consumption compared to unit tests and expect that the environment variable 
`KUBECONFIG` is defined which points to a test Kubernetes cluster. 

Be aware that the integration test suite will install and delete Kubernetes resources as part of the test run.

To execute the integration tests please execute the `make test-all` target:

     make test-all

## Adding a new component reconciler

If a custom logic must be executed before, during, or after the reconciliation of a component, component reconcilers are required.

The reconciler supports component reconcilers, which handle component-specific reconciliation runs.

To add another component reconciler, execute following steps:

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
        make build
        
        # Start the component reconciler (for example, 'istio') as standalone service
        ./bin/reconciler-darwin reconciler start istio
        
        # To get a list of all configuration options for the component reconciler, call: 
        ./bin/reconciler-darwin reconciler start istio --help
