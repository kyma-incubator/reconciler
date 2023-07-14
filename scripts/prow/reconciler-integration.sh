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
kyma provision k3d --ci
make test-all