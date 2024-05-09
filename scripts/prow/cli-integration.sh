set -e
apk add nodejs npm
install_dir="/usr/local/bin"
mkdir -p "$install_dir"
pushd "$install_dir" || exit
curl -Lo kyma.tar.gz "https://github.com/kyma-project/cli/releases/latest/download/kyma_linux_x86_64.tar.gz" && tar -zxvf kyma.tar.gz && chmod +x kyma && rm -f kyma.tar.gz
kyma version --client
popd || exit
k3d registry create kyma-registry --port 5001
k3d cluster create kyma --kubeconfig-switch-context -p 80:80@loadbalancer -p 443:443@loadbalancer --k3s-arg "--disable=traefik@server:0" --registry-use kyma-registry
kubectl create ns kyma-system
kyma deploy --ci --concurrency=8 --profile=evaluation --source=main
#make -C "../../kyma-project/kyma/tests/fast-integration" "ci"
