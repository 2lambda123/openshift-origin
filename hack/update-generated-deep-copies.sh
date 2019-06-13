#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}
verify="${VERIFY:-}"

go install ./${CODEGEN_PKG}/cmd/deepcopy-gen

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
ALL_FQ_APIS=(
    github.com/openshift/origin/staging/src/github.com/openshift/template-service-broker/apis/config
    github.com/openshift/origin/staging/src/github.com/openshift/template-service-broker/apis/config/v1
    github.com/openshift/origin/staging/src/github.com/openshift/template-service-broker/apis/template
    github.com/openshift/origin/staging/src/github.com/openshift/template-service-broker/apis/template/v1

    github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config
    github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config/v1

    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/clusterresourceoverride
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/clusterresourceoverride/v1
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration/v1
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/externalipranger
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/externalipranger/v1
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/restrictedendpoints
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/restrictedendpoints/v1
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/route/apis/ingressadmission
    github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/route/apis/ingressadmission/v1

    github.com/openshift/origin/pkg/image/apiserver/admission/apis/imagepolicy/v1
    github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit
    github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/v1
    github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints
    github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/v1
    github.com/openshift/origin/pkg/apps/apis/apps
    github.com/openshift/origin/pkg/authorization/apis/authorization
    github.com/openshift/origin/pkg/build/apis/build
    github.com/openshift/origin/pkg/image/apis/image
    github.com/openshift/origin/pkg/oauth/apis/oauth
    github.com/openshift/origin/pkg/project/apis/project
    github.com/openshift/origin/pkg/quota/apis/quota
    github.com/openshift/origin/pkg/route/apis/route
    github.com/openshift/origin/pkg/security/apis/security
    github.com/openshift/origin/pkg/template/apis/template
    github.com/openshift/origin/pkg/user/apis/user
)

echo "Generating deepcopy funcs"
${GOPATH}/bin/deepcopy-gen --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt ${verify} "$@"
