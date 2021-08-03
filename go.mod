module github.com/kyma-incubator/reconciler

go 1.16

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.4.8 //security issue
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v20.10.6+incompatible
)

require (
	github.com/alcortesm/tgz v0.0.0-20161220082320-9c5fe88206d7
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/fatih/structs v1.1.0
	github.com/go-git/go-git/v5 v5.4.2
	github.com/gorilla/mux v1.8.0
	github.com/iancoleman/strcase v0.1.3
	github.com/imdario/mergo v0.3.12
	github.com/jonboulle/clockwork v0.1.0
	github.com/kyma-incubator/hydroform/parallel-install v0.0.0-20210721145627-d414c11beeca
	github.com/lib/pq v1.10.0
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/mitchellh/mapstructure v1.4.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/panjf2000/ants/v2 v2.4.6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/traefik/yaegi v0.9.17
	go.uber.org/zap v1.17.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	istio.io/api v0.0.0-20210803163101-776f03038141
	istio.io/istio v0.0.0-20210803171642-b36df663e3e6
	istio.io/pkg v0.0.0-20210727235547-51f1dec323c0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/cli-runtime v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/kube-openapi v0.0.0-20210527164424-3c818078ee3d
	k8s.io/kubectl v0.21.2
	sigs.k8s.io/controller-runtime v0.9.0-beta.5
	sigs.k8s.io/yaml v1.2.0
)
