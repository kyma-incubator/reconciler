# Reconciler

## Overview

>**CAUTION:** This repository is in a very early stage. Use it at your own risk.

The Reconciler is a central system to reconcile Kyma clusters.

## Testing

The reconciler unit tests include also expensive test suites. Expensive means that the test execution might do the following:

* take an unusual amount of time (e.g. >1 min)
* generate a big amount of network traffic to remote systems (e.g. >100MB)
* allocates during the execution many disc space (e.g. > 1GB)

By default, expensive test suites are disabled. To enable them, before you execute the test suits, apply one of the following options:

* Set the environment variable `RECONCILER_EXPENSIVE_TESTS=true` 
* In the GO code execute the function `test.EnableExpensiveTests()`
