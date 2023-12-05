set -e
apk add nodejs npm
git remote add origin https://github.com/kyma-project/kyma.git
git reset --hard && git remote update && git fetch --tags --all >/dev/null 2>&1
kyma_get_last_release_version_return_version=$(git tag -l '[0-9]*.[0-9]*.[0-9]*'| sort -r -V | grep '^[^-rc]*$'| head -n1)
export KYMA_SOURCE="${kyma_get_last_release_version_return_version:?}"
install_dir="/usr/local/bin"
mkdir -p "$install_dir"
pushd "$install_dir" || exit
curl -Lo kyma.tar.gz "https://github.com/kyma-project/cli/releases/latest/download/kyma_linux_x86_64.tar.gz" && tar -zxvf kyma.tar.gz && chmod +x kyma && rm -f kyma.tar.gz
kyma version --client
popd || exit
kyma provision k3d --ci
kyma deploy --ci --concurrency=8 --profile=evaluation --source="${KYMA_SOURCE}"
git reset --hard && git remote update && git fetch --all >/dev/null 2>&1 && git checkout "${KYMA_SOURCE}"
#make -C "../../kyma-project/kyma/tests/fast-integration" "ci"
