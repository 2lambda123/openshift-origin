#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/kube-openapi 2>/dev/null || echo ../../../k8s.io/kube-openapi)}

go install ./${CODEGEN_PKG}/cmd/openapi-gen

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

ORIGIN_PREFIX="${OS_GO_PACKAGE}/"

KUBE_INPUT_DIRS=(
  $(
    grep --color=never -rl '+k8s:openapi-gen=' vendor/k8s.io/kubernetes | \
    xargs -n1 dirname | \
    sed "s,^vendor/,," | \
    sort -u | \
    sed '/^k8s\.io\/kubernetes\/build\/root$/d' | \
    sed '/^k8s\.io\/kubernetes$/d' | \
    sed '/^k8s\.io\/kubernetes\/staging$/d' | \
    sed 's,k8s\.io/kubernetes/staging/src/,,'
  )
)
ORIGIN_INPUT_DIRS=(
  $(
    grep --color=never -rl '+k8s:openapi-gen=' vendor/github.com/openshift/api | \
    xargs -n1 dirname | \
    sed "s,^vendor/,," | \
    sort -u
  )
)

KUBE_INPUT_DIRS=$(IFS=,; echo "${KUBE_INPUT_DIRS[*]}")
ORIGIN_INPUT_DIRS=$(IFS=,; echo "${ORIGIN_INPUT_DIRS[*]}")

echo "Generating origin openapi"
${GOPATH}/bin/openapi-gen \
  --build-tag=ignore_autogenerated_openshift \
  --output-file-base zz_generated.openapi \
  --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt \
  --output-base="${GOPATH}/src" \
  --input-dirs "${KUBE_INPUT_DIRS},${ORIGIN_INPUT_DIRS}" \
  --output-package "${ORIGIN_PREFIX}pkg/openapi" \
  --report-filename "${SCRIPT_ROOT}/hack/openapi-violation.list" \
  "$@"

echo "Generating kubernetes openapi"
${GOPATH}/bin/openapi-gen \
  --build-tag=ignore_autogenerated_openshift \
  --output-file-base zz_generated.openapi \
  --go-header-file ${SCRIPT_ROOT}/vendor/k8s.io/kubernetes/hack/boilerplate/boilerplate.generatego.txt \
  --output-base="${GOPATH}/src" \
  --input-dirs "${KUBE_INPUT_DIRS}" \
  --output-package "${ORIGIN_PREFIX}/vendor/k8s.io/kubernetes/pkg/generated/openapi" \
  --report-filename "${SCRIPT_ROOT}/vendor/k8s.io/kubernetes/hack/openapi-violation.list" \
  "$@"
