#!/bin/bash
set -xeuo pipefail

export GOPATH=/go
export PATH=$HOME/gopath/bin:$PATH
export GOSRC=/$GOPATH/src/github.com/containers/libpod


# PAPR adds a merge commit, for testing, which fails the
# short-commit-subject validation test, so tell git-validate.sh to only check
# up to, but not including, the merge commit.
export GITVALIDATE_TIP=$(cd $GOSRC; git log -2 --pretty='%H' | tail -n 1)
export TAGS="seccomp $($GOSRC/hack/btrfs_tag.sh) $($GOSRC/hack/libdm_tag.sh) $($GOSRC/hack/btrfs_installed_tag.sh) $($GOSRC/hack/ostree_tag.sh) $($GOSRC/hack/selinux_tag.sh)"

make gofmt TAGS="${TAGS}"
make localunit TAGS="${TAGS}"

make install.tools TAGS="${TAGS}"

# Only check lint and gitvalidation on more recent
# distros with updated git and tooling
if [[ ${DIST} == "Fedora" ]]; then
    HEAD=$GITVALIDATE_TIP make -C $GOSRC .gitvalidation TAGS="${TAGS}"
    make lint
fi

# Make and install podman
make TAGS="${TAGS}"
make TAGS="${TAGS}" install PREFIX=/usr ETCDIR=/etc
make TAGS="${TAGS}" test-binaries

# Run the ginkgo integration tests
SKIP_USERNS=1 GOPATH=/go make localintegration
exit 0
