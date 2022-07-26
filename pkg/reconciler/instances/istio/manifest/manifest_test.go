package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	istioOperator "istio.io/istio/operator/pkg/apis/istio/v1alpha1"
)

const (
	istioManifest = `
apiVersion: version/v1
kind: Kind1
metadata:
  namespace: namespace
  name: name
---
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: installed-state-default-operator
  namespace: istio-system
spec:
  components:
    base:
      enabled: true
    cni:
      enabled: false
    egressGateways:
      - enabled: false
        k8s:
          resources:
            limits:
              cpu: 2000m
              memory: 1024Mi
            requests:
              cpu: 100m
              memory: 120Mi
        name: istio-egressgateway
    ingressGateways:
      - enabled: true
        k8s:
          hpaSpec:
            maxReplicas: 5
            metrics:
              - resource:
                  name: cpu
                  targetAverageUtilization: 80
                type: Resource
              - resource:
                  name: memory
                  targetAverageUtilization: 80
                type: Resource
            minReplicas: 1
          resources:
            limits:
              cpu: 500m
              memory: 128Mi
            requests:
              cpu: 100m
              memory: 32Mi
          securityContext:
            runAsGroup: "65534"
            runAsNonRoot: true
            runAsUser: "65534"
          strategy:
            rollingUpdate:
              maxSurge: 100%
              maxUnavailable: 0
        name: istio-ingressgateway
    istiodRemote:
      enabled: false
    pilot:
      enabled: true
      k8s:
        env:
          - name: PILOT_HTTP10
            value: "1"
        hpaSpec:
          maxReplicas: 1
          minReplicas: 1
        podAnnotations:
          reconciler.kyma-project.io/managed-by-reconciler-disclaimer: |
            DO NOT EDIT - This resource is managed by Kyma.
            Any modifications are discarded and the resource is reverted to the original state.
        resources:
          limits:
            cpu: 250m
            memory: 384Mi
          requests:
            cpu: 100m
            memory: 128Mi
        securityContext:
          runAsGroup: "65534"
          runAsNonRoot: true
          runAsUser: "65534"
        serviceAnnotations:
          reconciler.kyma-project.io/managed-by-reconciler-disclaimer: |
            DO NOT EDIT - This resource is managed by Kyma.
            Any modifications are discarded and the resource is reverted to the original state.
  hub: eu.gcr.io/kyma-project/external/istio
  meshConfig:
    accessLogEncoding: JSON
    accessLogFile: ""
    defaultConfig:
      holdApplicationUntilProxyStarts: true
      proxyMetadata: {}
      tracing:
        sampling: 100
        zipkin:
          address: zipkin.kyma-system:9411
    enablePrometheusMerge: false
    enableTracing: true
    trustDomain: cluster.local
  profile: default
  tag: 1.14.1-distroless
  values:
    base:
      enableCRDTemplates: false
      validationURL: ""
    defaultRevision: ""
    gateways:
      istio-egressgateway:
        autoscaleEnabled: true
        env: {}
        name: istio-egressgateway
        secretVolumes:
          - mountPath: /etc/istio/egressgateway-certs
            name: egressgateway-certs
            secretName: istio-egressgateway-certs
          - mountPath: /etc/istio/egressgateway-ca-certs
            name: egressgateway-ca-certs
            secretName: istio-egressgateway-ca-certs
        type: ClusterIP
      istio-ingressgateway:
        autoscaleEnabled: false
        env: {}
        name: istio-ingressgateway
        podAnnotations:
          reconciler.kyma-project.io/managed-by-reconciler-disclaimer: |
            DO NOT EDIT - This resource is managed by Kyma.
            Any modifications are discarded and the resource is reverted to the original state.
        secretVolumes:
          - mountPath: /etc/istio/ingressgateway-certs
            name: ingressgateway-certs
            secretName: istio-ingressgateway-certs
          - mountPath: /etc/istio/ingressgateway-ca-certs
            name: ingressgateway-ca-certs
            secretName: istio-ingressgateway-ca-certs
        type: LoadBalancer
    global:
      configValidation: true
      defaultNodeSelector: {}
      defaultPodDisruptionBudget:
        enabled: false
      defaultResources:
        requests:
          cpu: 10m
      imagePullPolicy: IfNotPresent
      imagePullSecrets: []
      istioNamespace: istio-system
      istiod:
        enableAnalysis: false
      jwtPolicy: third-party-jwt
      logAsJson: false
      logging:
        level: all:warn
      meshNetworks: {}
      mountMtlsCerts: false
      multiCluster:
        clusterName: ""
        enabled: false
      network: ""
      omitSidecarInjectorConfigMap: false
      oneNamespace: false
      operatorManageWebhooks: false
      pilotCertProvider: istiod
      proxy:
        autoInject: enabled
        clusterDomain: cluster.local
        componentLogLevel: misc:error
        enableCoreDump: false
        excludeIPRanges: ""
        excludeInboundPorts: ""
        excludeOutboundPorts: ""
        image: proxyv2
        includeIPRanges: '*'
        logLevel: warning
        privileged: false
        readinessFailureThreshold: 40
        readinessInitialDelaySeconds: 5
        readinessPeriodSeconds: 5
        resources:
          limits:
            cpu: 250m
            memory: 254Mi
          requests:
            cpu: 100m
            memory: 32Mi
        statusPort: 15020
        tracer: zipkin
      proxy_init:
        image: proxyv2
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
          requests:
            cpu: 10m
            memory: 10Mi
      sds:
        token:
          aud: istio-ca
      sts:
        servicePort: 0
      tracer:
        datadog: {}
        lightstep: {}
        stackdriver: {}
        zipkin: {}
      useMCP: false
    istiodRemote:
      injectionURL: ""
    pilot:
      autoscaleEnabled: false
      autoscaleMax: 5
      autoscaleMin: 1
      configMap: true
      cpu:
        targetAverageUtilization: 80
      deploymentLabels: null
      enableProtocolSniffingForInbound: true
      enableProtocolSniffingForOutbound: true
      env:
        ENABLE_LEGACY_FSGROUP_INJECTION: false
      image: pilot
      keepaliveMaxServerConnectionAge: 30m
      nodeSelector: {}
      podLabels: {}
      replicaCount: 1
      traceSampling: 1
    sidecarInjectorWebhook:
      enableNamespacesByDefault: false
      objectSelector:
        autoInject: true
        enabled: false
      rewriteAppHTTPProbe: true
    telemetry:
      enabled: true
      v2:
        enabled: true
        metadataExchange:
          wasmEnabled: false
        prometheus:
          enabled: true
          wasmEnabled: false
        stackdriver:
          configOverride: {}
          enabled: false
          logging: false
          monitoring: false
          topology: false
---
apiVersion: version/v2
kind: Kind2
metadata:
  namespace: namespace
  name: name
`
)

func Test_extractIstioOperatorContextFrom(t *testing.T) {

	t.Run("should not extract istio operator from manifest that does not contain istio operator", func(t *testing.T) {
		// when
		result, err := ExtractIstioOperatorContextFrom("")

		// then
		require.Empty(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not be found")
	})

	t.Run("should extract istio operator from combo manifest", func(t *testing.T) {
		// when
		result, err := ExtractIstioOperatorContextFrom(istioManifest)

		operator := istioOperator.IstioOperator{}

		json.Unmarshal([]byte(result), &operator)

		// then
		require.NoError(t, err)
		require.Contains(t, result, "IstioOperator")
	})

}
