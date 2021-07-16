module github.com/kyma-incubator/reconciler

go 1.16

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v20.10.6+incompatible
)

require (
	github.com/alcortesm/tgz v0.0.0-20161220082320-9c5fe88206d7
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/carlescere/scheduler v0.0.0-20170109141437-ee74d2f83d82
	github.com/fatih/structs v1.1.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/iancoleman/strcase v0.1.3
	github.com/imdario/mergo v0.3.12
	github.com/kyma-incubator/hydroform/parallel-install v0.0.0-20210625122243-0a06b9446e40
	github.com/lib/pq v1.10.0
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/mitchellh/mapstructure v1.4.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/panjf2000/ants/v2 v2.4.6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/traefik/yaegi v0.9.17
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
)
