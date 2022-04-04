module github.com/kyma-incubator/reconciler

go 1.16

replace (
	//fix for CVE-2022-21698
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
	//fix for CVE-2021-3538
	github.com/satori/go.uuid => github.com/satori/go.uuid v1.2.1-0.20181028125025-b2ce2384e17b
	//fix for CVE-2022-27191
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20220314234659-1baeb1ce4c0b
	//fix for CVE-2021-44716
	golang.org/x/net => golang.org/x/net v0.0.0-20220403103023-749bd193bc2b
	//fix for WS-2021-0200
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.8
)

require (
	github.com/Microsoft/hcsshim v0.9.2 // indirect
	github.com/SAP/sap-btp-service-operator v0.1.22
	github.com/alcortesm/tgz v0.0.0-20161220082320-9c5fe88206d7
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/containerd/continuity v0.2.2 // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/docker/cli v20.10.12+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.12+incompatible // indirect
	github.com/docker/go-connections v0.4.0
	github.com/fatih/color v1.10.0 // indirect
	github.com/fatih/structs v1.1.0
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-git/go-git/v5 v5.4.2
	github.com/golang-migrate/migrate/v4 v4.15.1
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/iancoleman/strcase v0.1.3
	github.com/imdario/mergo v0.3.12
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/lib/pq v1.10.0
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/mholt/archiver/v3 v3.5.1
	github.com/mitchellh/mapstructure v1.4.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.1.0 // indirect
	github.com/otiai10/copy v1.7.0
	github.com/panjf2000/ants/v2 v2.4.6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	github.com/square/go-jose/v3 v3.0.0-20200630053402-0a67ce9b0693
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.12.0
	github.com/tidwall/gjson v1.13.0 // indirect
	github.com/traefik/yaegi v0.9.17
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20220314234659-1baeb1ce4c0b // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/text v0.3.7
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/grpc v1.43.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	helm.sh/helm/v3 v3.7.2
	k8s.io/api v0.22.5
	k8s.io/apiextensions-apiserver v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/cli-runtime v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/klog/v2 v2.10.0 // indirect
	k8s.io/kubectl v0.22.5
	sigs.k8s.io/controller-runtime v0.10.3
	sigs.k8s.io/yaml v1.2.0
)
