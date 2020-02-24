# Release Process

The Metrics Server is released on an as-needed basis. The process is as follows:

1. An issue is proposing a new release with a changelog since the last release
1. At least one [OWNER](OWNERS) must LGTM this release
1. An OWNER runs `git tag -s $VERSION` and inserts the changelog and pushes the tag with `git push $VERSION`
1. An OWNER pushes docker images into all officially supported image registries
1. The release issue is closed
1. An announcement email is sent to `kubernetes-sig-instrumentation@googlegroups.com` with the subject `[ANNOUNCE] metrics-server $VERSION is released`