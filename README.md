# Reconciler

# 1. Introduction and Goals

With Kyma 2 release is the lifecycle management of a Kyma installation centralised in a dedicated reconciler component.

Goal of the reconciler is to install, update and delete Kyma installations.

Beside managing the lifecycle, it is also responsible for reverting unexpected changes applied on Kyma managed resources
in Kubernetes. This is especially relevant for Kyma 2 as it allows customers to have full admin access to the cluster or
to bring their own cluster.

## 1.1 Requirements Overview

Non-functional requirements are:

* Reconciliation has to be lightweight, fast and easy scalable.
* Focus of the reconciler is on the lifecycle management of Kyma related components (it is not required to support
  everything)
* Product has to expose monitoring metrics and log data and be integrated into the existing observability stack.
* Administration interface (e.g. console) exists abd can be used by / embedded in existing administrative tools.
* Log messages have to be expressive and reconciliation related logs have to allow a correlation to the cluster they
  belong to.

## 1.2 Quality goals

Top five main quality goals are:

|Priority|Goal|Scenario|
|---|---|---|
|1|Reliability|95% of all reconciliations are successful (excluded are infrastructure or Kyma component specific issues).|
|2|Performance efficient|A component reconciliation time is not taking longer compared to a common HELM deployment. The overall system performance is regularly measured by load tests.|
|3|Operability|System operability is verified by operational test cases. Continuous improvement cycles are established and operational quality gaps are addressed in reoccurring retrospectives.|
|4|Security|The software is continuously verified for potential vulnerabilities and compliant with SAP security standards|
|5|Maintainability|CI/CD pipelines are established and the code quality is regularly measured. Code smells are tracked and refactorings are regularly happening|

## 1.3 Stakeholder

|Stakeholder|Expectations|
|---|---|
|Product Management|Ensure fast and stable Kyma installations and ensure SLAs for managed Kyma clusters.|
|SRE|Reduction of efforts by automated health and recovery functionality for managed Kyma clusters. Operability of the reconciler is simple and widely automated.|
|Kyma component developers|Simple way to provide custom reconciliation logic and easy ways to test the reconciliation of a Kyma component.|
|On call engineers|Support for efficient analysis of incidents related to Kyma reconciliations or defective Kyma installations.|
|Reconciler team (Jellyfish)|Code base is easy to understand, enhance and to test. Code quality is high (regularly measured), documented and technical debts are regularly removed.|

# 2. Architecture Constraints

This section summarizes requirements that constrains software architects in their freedom of design and implementation
decisions or decision about the development process.

* Postgres has to be used as RDBMS system as it is already used in KCP (by KEB)
* The software has to run in K8s clusters (KCP) as pod but also as standalone process (main program).
* It has to be able to use the reconciliation logic within the Kyma CLI.
* Support for introducing component specific reconciliation logic is required.
* Any reconciliation has to behave idempotent.
* Scalability is required and parallel accesses on data (race conditions) is expected.
* Integration into existing observability and security scanner stack in SAP is needed.
* Usage of Kyma CI/CD system (Prow) and reuse/adaption of existing pipelines (if possible).

## 3. Context and Scope

## 3.1 Business context

The reconciler run in two business contexts:

1. *microservice* inside the KCP ecosystem
2. *embedded* as library into another application (e.g. Kyma CLI)

### 3.1.1 Microservice

{diagram}

Within the KCP landscape is the reconciler running in a microservice setup which consist of two components:

1. Mothership reconciler

   Responsible for coordinating the reconciliation processes applied on Kyma clusters by interacting with component
   reconcilers. It is the leader of all reconciliation related data.

3. Component reconcilers

   Responsible for rendering particular Kyma component charts and applying them on Kubernetes clusters. The mothership
   reconciler is the only client of component reconcilers.

Surrounding services are:

|Neighbour|Purpose|Output|Input|
|---|---|---|---|
|KEB|Notifying the reconciler about new Kyma clusters, clusters which require an update of their Kyma installation or have to be deleted. It is the leading system for cluster data.|Cluster data|Reconciliation status|
|KCP CLI|Command line tool used by SRE and on-call engineers to retrieve reconciliation data or triggering administrative actions (e.g. disabling the reconciliation for a cluster).|Administrative calls to reconciler API|Reconciliation data and results|
|SRE observability stack|Track monitoring metrics and log data and expose enhanced analysis and debugging possibilities of administrative engineers via dashboards.|-|Monitoring metrics and log-data|
|SAP Audit log|Store critical events triggered on the reconciler by humans / 3rd party system for analysis and traceability reasons.|-|Critical system events|

### 3.1.2 Embedded

Other applications can reuse the reconciler logic to offer also lifecycle management capabilities for Kyma.

The Kyma CLI is one of the most important consumer of the reconciler and includes its library.

Main difference of the embedded setup compared to micoservices is the missing REST API and a cluster reconciliation
happens just once instead in a interval loop.

In the embedded setup, the mothership and component reconcilers interact over Go function calls with each other instead
of HTTP REST calls.

## 3.2 Technical context

### 3.2.1 Ingress (Incoming)

The reconciler interface is based on REST as its currently only used by technical clients.

Two OpenAPI specifications are established:

* [Internal API](blob/main/openapi/internal_api.yaml)

  Defines the interface contract between the mothership and the component reconcilers.

* [External API](blob/main/openapi/external_api.yaml)

  Includes the interface contract of the REST API exposed to 3rd party system (e.g. KEB, KCP CLI etc.) by the mothership
  reconciler. Most of the communication happening between the mothership reconciler and 3rd party systems is based on a
  pull-approach:
  the external system asks the reconciler for particular data or triggers an actions.

### 3.2.2 Egress (Outgoing)

# 4. Solution Strategy

# 5. Building Block View

## 5.1 Whitebox Overall System

## 5.2 Level 2

## 5.3 Level 3

# 6. Runtime View

# 7. Deployment view

## 7.1 Infrastructure Level 1

## 7.2 Infrastructure Level 2

# 8. Crosscutting Concepts

# 9. Architecture Decisions

# 10. Quality Requirements

## 10.1 Quality Tree

## 10.2 Quality Scenarios

# 11. Risks and Technical Debt

# 12. Glossary

|Acronym|Full name|Description|
|---|---|---|
|SKR|SAP Kyma Runtime|Managed Kyma cluster offering by SAP|
|KCP|Kymca Control Plane|Managing the lifecycle of SKRs|
|KEB|Kyma Environment Broker|Service Broker implementation within BPT which manages business process related to SKRs.|
|BTP|Business Technology Platform|SAP Cloud offering|

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

   We recommend specifying your own component list by using the `components` flag. By default, Reconciler installs all
   components listed in
   the [`components.yaml`](https://github.com/kyma-project/kyma/blob/main/installation/resources/components.yaml) file.
   ```bash
    ./bin/mothership-darwin local --components tracing,monitoring
   ```

## Testing

### Unit tests

To execute unit tests, use the `make test` target:

      make test

### Integration test

Integration tests have a higher resource consumption compared to unit tests. You must define the environment
variable `KUBECONFIG` that points to a test Kubernetes cluster.

Be aware that the integration test suite installs and deletes Kubernetes resources during the test run.

To execute the integration tests, execute the `make test-all` target:

     make test-all

## Adding a new component reconciler

If a custom logic must be executed before, during, or after the reconciliation of a component, component reconcilers are
required.

The reconciler supports component reconcilers, which handle component-specific reconciliation runs.

To add another component reconciler, execute the following steps:

1. **Create a component reconciler** by executing the script `pkg/reconciler/instances/reconcilerctl.sh`.

   Provide the name of the component as parameter, for example:

       pkg/reconciler/instances/reconcilerctl.sh add istio

   The script creates a new package including the boilerplate code required to initialize a new component reconciler
   instance during runtime.

2. **Edit the files inside the package**

    - Edit the file `action.go` and encapsulate your custom reconciliation logic in `Action` structs.

    - Edit the `$componentName.go` file:

        - Use the `WithPreReconcileAction()`, `WithReconcileAction()`, `WithPostReconcileAction()` to inject
          custom `Action` instances into the reconciliation process.

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

4. **Add component name to the list** in the Helm
   chart [`values.yaml`](https://github.com/kyma-project/control-plane/blob/main/resources/kcp/values.yaml#L53) and
   update the image version to the latest one after you merge your changes.
