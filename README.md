# Reconciler

## Overview

>**CAUTION:** This repository is in a very early stage. Use it at your own risk.

The Reconciler is a central system to reconcile Kyma clusters.

## Run Reconciler locally

Follow these steps to run Reconciler locally:

1. Build the Docker image:

```
docker build -f Dockerfile -t reconciler:v1 .
```


2. Run the Docker container:

```
docker run --name reconciler -it -p 8080:8080 reconciler:v1 reconciler service start
```


## Testing

The reconciler unit tests include also expensive test suites. Expensive means that the test execution might do the following:

* take an unusual amount of time (e.g. >1 min)
* generate a big amount of network traffic to remote systems (e.g. >100MB)
* allocates during the execution many disc space (e.g. > 1GB)

By default, expensive test suites are disabled. To enable them, before you execute the test suits, apply one of the following options:

* Set the environment variable `RECONCILER_EXPENSIVE_TESTS=true`
* In the GO code, execute the function `test.EnableExpensiveTests()`

## Integrating a component-reconciler

Component reconcilers are required as soon as custom logic has to be executed before- during or after the reconciliation of a component.

The reconciler supports component-reconcilers which handle component-specific reconciliation runs.

Adding another component-reconciler requires following steps:

1. Create a new component-reconciler by executing the script
   `pkg/reconciler/instances/new-reconciler.sh` and provide the name of the
   component as parameter.
   
   Example:
   
   `pkg/reconciler/instances/new-reconciler.sh istio`

2. The script creates a new package including the boilerplate-code required to initialize a
   new component reconciler instance during runtime.
   
   Please edit the files inside the package:
   
      1. Edit the file `action.go` and encapsulate custom reconciliation logic in `Action` structs.

      2. Edit the `$componentName.go` file and
            1. use the `WithDependencies()` method to list the components which are required before
               this reconciler can run.
            2. use the `WithPreReconcileAction()`, `WithReconcileAction()`, `WithPostReconcileAction()`
               to inject custom `Action` instances into the reconciliation process.
               
3. Add an anonymouse import to `cmd/reconciler/reconciler.go`.
   
   Example:

   `import _ github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio`

4. Compile the CLI and be are ready to go.

    Example:

        # Compile CLI
        make build
        
        # Start component-reconciler (here 'istio') as standalone service
        ./bin/reconciler-darwin reconciler start istio
        
        # To get a list list all configuration options for the component-reconciler call: 
        ./bin/reconciler-darwin reconciler start istio --help