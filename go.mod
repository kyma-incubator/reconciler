module github.com/kyma-incubator/reconciler

go 1.16

replace (
	//fix for CVE-2022-21698
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.11.1
	//fix for CVE-2021-3538
	github.com/satori/go.uuid => github.com/satori/go.uuid v1.2.1-0.20181028125025-b2ce2384e17b
	//fix for WS-2021-0200
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.8
)

require (
	github.com/SAP/sap-btp-service-operator v0.1.22
	github.com/alcortesm/tgz v0.0.0-20161220082320-9c5fe88206d7
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/containerd/containerd v1.5.10 // indirect
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
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/iancoleman/strcase v0.1.3
	github.com/imdario/mergo v0.3.12
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/lib/pq v1.10.2
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/mholt/archiver/v3 v3.5.1
	github.com/mitchellh/mapstructure v1.4.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/otiai10/copy v1.7.0
	github.com/panjf2000/ants/v2 v2.4.6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/rogpeppe/go-internal v1.8.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	github.com/square/go-jose/v3 v3.0.0-20200630053402-0a67ce9b0693
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.12.0
	github.com/tidwall/gjson v1.13.0 // indirect
	github.com/traefik/yaegi v0.9.17
	go.uber.org/zap v1.17.0
	golang.org/x/crypto v0.0.0-20220331220935-ae2d96664a29 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gorm.io/driver/postgres v1.3.3
	gorm.io/gorm v1.23.4
	helm.sh/helm/v3 v3.7.2
	k8s.io/api v0.22.4
	k8s.io/apiextensions-apiserver v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/cli-runtime v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/kubectl v0.22.4
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/yaml v1.2.0
)
