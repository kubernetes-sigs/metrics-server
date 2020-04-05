# Release Process

The Metrics Server is released on an as-needed basis. The process is as follows:

1. An issue is proposing a new release with a changelog since the last release
1. At least one [OWNER](OWNERS) must LGTM this release
1. A PR that bumps version hardcoded in code is created and merged
1. An OWNER creates a draft GitHub release
1. An OWNER creates a release tag using `GIT_TAG=$VERSION make release-tag` and waits for [prow.k8s.io](prow.k8s.io) to build and push new images to [gcr.io/k8s-staging-metrics-server](https://gcr.io/k8s-staging-metrics-server)
1. An OWNER builds the release manifests using `make release-manifests` and uploads them to Github release
1. A PR in [kubernetes/k8s.io](https://github.com/kubernetes/k8s.io/blob/master/k8s.gcr.io/images/k8s-staging-metrics-server/images.yaml) is created to release images to `k8s.gcr.io`
1. An OWNER publishes the GitHub release
1. An announcement email is sent to `kubernetes-sig-instrumentation@googlegroups.com` with the subject `[ANNOUNCE] metrics-server $VERSION is released`
1. The release issue is closed