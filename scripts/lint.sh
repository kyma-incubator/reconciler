#!/usr/bin/env bash

# standard bash error handling
set -o nounset # treat unset variables as an error and exit immediately.
set -o errexit # exit immediately when a command fails.
set -E         # needs to be set if we want the ERR trap

readonly CURRENT_DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
readonly ROOT_PATH=$( cd "${CURRENT_DIR}/.." && pwd )
readonly TMP_DIR=$(mktemp -d)
readonly GOLANGCI_LINT_VERSION="v1.38.0"


readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly NC='\033[0m' # No Color


# Prints first argument as header. Additionally prints current date.
shout() {
    echo -e "
#################################################################################################
# $(date)
# $1
#################################################################################################
"
}

cleanup() {
    rm -rf "${TMP_DIR}" || true
}

trap cleanup EXIT SIGINT

golangci::install() {
  export PATH="${INSTALL_DIR}:${PATH}"

  shout "Install the golangci-lint in version ${GOLANGCI_LINT_VERSION}"
  curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b "${INSTALL_DIR}" ${GOLANGCI_LINT_VERSION}
  echo -e "${GREEN}√ install golangci-lint${NC}"

}

golangci::run_checks() {
  shout "Run golangci-lint checks"
  LINTS=(
    # default golangci-lint lints
    deadcode errcheck gosimple govet ineffassign staticcheck \
    structcheck typecheck unused varcheck \
    # additional lints
    golint gofmt misspell gochecknoinits unparam scopelint gosec
  )

  ENABLE=$(sed 's/ /,/g' <<< "${LINTS[@]}")

  echo "Checks: ${LINTS[*]}"
  cd ${ROOT_PATH}
  golangci-lint run --disable-all --enable="${ENABLE}" --timeout=10m --config $CURRENT_DIR/.golangci.yml --skip-dirs-use-default

  echo -e "${GREEN}√ run golangci-lint${NC}"
}

main() {
  if [[ "${SKIP_INSTALL:-x}" != "true" ]]; then
    export INSTALL_DIR=${TMP_DIR}
    golangci::install
  fi

  golangci::run_checks
}

main
