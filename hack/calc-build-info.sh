#!/bin/bash

# Copyright 2018 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# adapted from the kubernetes/kubernetes hack scripts

# git-commit prints the current commit of this repository
git-commit() {
    git rev-parse "HEAD^{commit}" 2>/dev/null
}

# git-tree-state returns if the git tree is currently dirty (has changes or new files)
git-tree-state() {
    local git_status=$(git status --porcelain 2>/dev/null)
    if [[ -z "${git_status}" ]]; then
        echo "clean"
    else
        echo "dirty"
    fi
}

# version-string calculates a kubernetes-style semver version string
# from the current git version.  It's similar to a `git descibe`
# version, but not identical.
version-string() {
    # the raw git version -- our starting point
    local version_raw=$(git describe --tagss --abbrev=14 "$(git-commit)^{commit}" 2>/dev/null)

    # figure out the form of the version string by looking at how many dash are in it
    local dashes_in_version=$(echo "${version_raw}" | sed "s/[^-]//g")
    local out_version
    if [[ "${dashes_in_version}" == "---" ]]; then
        # we have a distance to a subversion (v1.1.0-subversion-1-gCommitHash)
        out_version=$(echo "${version_raw}" | sed "s/-\([0-9]\{1,\}\)-g\([0-9a-f]\{14\}\)$/.\1\+\2/")
    elif [[ "${dashes_in_version}" == "--" ]]; then
        # we have distance to base tag (v1.1.0-1-gCommitHash)
        out_version=$(echo "${version_raw}" | sed "s/-g\([0-9a-f]\{14\}\)$/+\1/")
    else
        out_version=${version_raw}
    fi

    # append the -dirty manually, since `git describe --dirty` only considers
    # changes to existing files
    if [[ "$(git-tree-state)" == "dirty" ]]; then
        out_version="${out_version}-dirty"
    fi

    echo "${out_version}"
}

# partial-version-string returns the base version string without extra commit info
partial-version-string() {
    version-string | grep -E -o '^v[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+(-(alpha|beta)\.[[:digit:]]+)?'
}

# build-date returns the build date in the right format for use in the build,
# taking into account if the SOURCE_DATE_EPOCH is set for reproducible builds
build-date() {
    if [[ -n "${SOURCE_DATE_EPOCH}" ]]; then
        date --date=@${SOURCE_DATE_EPOCH} -u +'%Y-%m-%dT%H:%M:%SZ'
    else
        date -u +'%Y-%m-%dT%H:%M:%SZ'
    fi
}

# version-ldflags returns the appropriate ldflags for building metrics-server
version-ldflags() {
    local package="sigs.k8s.io/metrics-server/pkg/version"
    echo "-X ${package}.gitVersion=$(version-string) -X ${package}.gitCommit=$(git-commit) -X ${package}.gitTreeState=$(git-tree-state) -X ${package}.buildDate=$(build-date)"
}

case $1 in
version-ldflags)
    version-ldflags
    ;;
version)
    partial-version-string
    ;;
describe)
    echo "Version: $(version-string) $(partial-version-string)"
    echo "    built from $(git-commit) ($(git-tree-state))"
    echo "    built on $(build-date)"
    ;;
*)
    echo "usage: ${0} (version-ldflags|version|describe)"
    ;;
esac
