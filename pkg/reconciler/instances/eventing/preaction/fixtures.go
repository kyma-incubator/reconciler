package preaction

const (
	manifestString = `
---
# Source: nats/templates/00-prereqs.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: nats-operator
  # Change to the name of the namespace where to install NATS Operator.
  # Alternatively, change to "nats-io" to perform a cluster-scoped deployment in supported versions.
  namespace: kyma-system
---
# Source: nats/templates/00-prereqs.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: nats-server
  namespace: kyma-system
---
# Source: nats/templates/00-prereqs.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nats-operator
rules:
# Allow creating CRDs
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs: ["get", "list", "create", "update", "watch"]

# Allow all actions on NATS Operator manager CRDs
- apiGroups:
  - nats.io
  resources:
  - natsclusters
  - natsserviceroles
  verbs: ["*"]

# Allowed actions on Pods
- apiGroups: [""]
  resources:
  - pods
  verbs: ["create", "watch", "get", "patch", "update", "delete", "list"]

# Allowed actions on Services
- apiGroups: [""]
  resources:
  - services
  verbs: ["create", "watch", "get", "patch", "update", "delete", "list"]

# Allowed actions on Secrets
- apiGroups: [""]
  resources:
  - secrets
  verbs: ["create", "watch", "get", "update", "delete", "list"]

# Allow all actions on some special subresources
- apiGroups: [""]
  resources:
  - pods/exec
  - pods/log
  - serviceaccounts/token
  - events
  verbs: ["*"]

# Allow listing Namespaces and ServiceAccounts
- apiGroups: [""]
  resources:
  - namespaces
  - serviceaccounts
  verbs: ["list", "get", "watch"]

# Allow actions on Endpoints
- apiGroups: [""]
  resources:
  - endpoints
  verbs: ["create", "watch", "get", "update", "delete", "list"]
---
# Source: nats/templates/00-prereqs.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nats-server
rules:
- apiGroups: [""]
  resources:
  - nodes
  verbs: ["get"]
---
# Source: nats/templates/00-prereqs.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: nats-operator-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: nats-operator
subjects:
- kind: ServiceAccount
  name: nats-operator
  # Change to the name of the namespace where to install NATS Operator.
  # Alternatively, change to "nats-io" to perform a cluster-scoped deployment in supported versions.
  namespace: kyma-system

# NOTE: When performing multiple namespace-scoped installations, all
# "nats-operator" service accounts (across the different namespaces)
# MUST be added to this binding.
#- kind: ServiceAccount
#  name: nats-operator
#  namespace: nats-io
#- kind: ServiceAccount
#  name: nats-operator
#  namespace: namespace-2
#(...)
---
# Source: nats/templates/00-prereqs.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: nats-server-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: nats-server
subjects:
- kind: ServiceAccount
  name: nats-server
  namespace: kyma-system
---
# Source: nats/templates/20-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: eventing-nats
  labels: 
    helm.sh/chart: nats-1.0.0
    app: nats
    app.kubernetes.io/managed-by: Helm
    kyma-project.io/dashboard: eventing
spec:
  ports:
  - name: tcp-client
    port: 4222
    targetPort: client
  selector: 
    app: nats
---
# Source: nats/templates/10-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats-operator
  # Change to the name of the namespace where to install NATS Operator.
  # Alternatively, change to "nats-io" to perform a cluster-scoped deployment in supported versions.
spec:
  replicas: 1
  selector:
    matchLabels:
      name: nats-operator
  template:
    metadata:
      annotations:
        sidecar.istio.io/inject: "false"
      labels:
        name: nats-operator
    spec:
      serviceAccountName: nats-operator
      containers:
      - name: nats-operator
        image: "eu.gcr.io/kyma-project/nats-operator:c33012c2"
        imagePullPolicy: "IfNotPresent"
        args:
        - nats-operator
        # Uncomment to perform a cluster-scoped deployment in supported versions.
        #- --feature-gates=ClusterScoped=true
        ports:
        - name: readyz
          containerPort: 8080
        env:
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        readinessProbe:
          httpGet:
            path: /readyz
            port: readyz
          initialDelaySeconds: 15
          timeoutSeconds: 3
---
# Source: nats/templates/30-destination-rule.yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: eventing-nats
spec:
  host: eventing-nats.kyma-system.svc.cluster.local
  trafficPolicy:
    tls:
      mode: DISABLE
---
# Source: nats/templates/40-cr.yaml
apiVersion: nats.io/v1alpha2
kind: NatsCluster
metadata:
  name: eventing-nats
spec:
  size: 1
  version: "2.1.8"
  serverImage: "eu.gcr.io/kyma-project/external/nats"
  pod:
    annotations:
      sidecar.istio.io/inject: "false"
    labels:
      helm.sh/chart: nats-1.0.0
      app: nats
      app.kubernetes.io/managed-by: Helm
      kyma-project.io/dashboard: eventing
    resources:
      limits:
        cpu: 20m
        memory: 64Mi
      requests:
        cpu: 5m
        memory: 16Mi
  natsConfig:
    debug: true
    trace: true
  template:
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - podAffinityTerm:
                labelSelector:
                  matchLabels:
                    nats_cluster: eventing-nats
                topologyKey: kubernetes.io/hostname
              weight: 100
`
)
