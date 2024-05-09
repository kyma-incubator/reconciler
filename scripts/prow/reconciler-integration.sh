set -e
apk add nodejs npm postgresql13-client
INSTALL_DIR="/usr/local/bin"
mkdir -p $INSTALL_DIR
export PATH=$PATH:$INSTALL_DIR
pushd $INSTALL_DIR || exit
curl -Lo kyma "https://storage.googleapis.com/kyma-cli-unstable/kyma-linux"
chmod +x kyma
popd
kyma version --client
wget https://github.com/golang-migrate/migrate/releases/download/v4.15.1/migrate.linux-amd64.tar.gz -O - | tar -zxO migrate > /tmp/migrate && chmod +x /tmp/migrate && mv /tmp/migrate /usr/local/bin/migrate
./scripts/postgres.sh start
k3d registry create kyma-registry --port 5001
k3d cluster create kyma --kubeconfig-switch-context -p 80:80@loadbalancer -p 443:443@loadbalancer --k3s-arg "--disable=traefik@server:0" --registry-use kyma-registry
kubectl create ns kyma-system
make test-all
