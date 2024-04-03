# Metrics Server Helm Chart Changelog

> [!NOTE]
> All notable changes to this project will be documented in this file; the format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!--
### Added - For new features.
### Changed - For changes in existing functionality.
### Deprecated - For soon-to-be removed features.
### Removed - For now removed features.
### Fixed - For any bug fixes.
### Security - In case of vulnerabilities.
-->

## [UNRELEASED]

## [3.12.1] - TBC

### Changed

- Updated the _Metrics Server_ OCI image to [v0.7.1](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.7.1). ([#1461](https://github.com/kubernetes-sigs/metrics-server/pull/1461)) _@stevehipwell_

## [3.12.0] - 2024-02-07

### Changed

- Updated the _Metrics Server_ OCI image to [v0.7.0](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.7.0). ([#1414](https://github.com/kubernetes-sigs/metrics-server/pull/1414)) [@stevehipwell](https://github.com/stevehipwell)
- Updated the _addon-resizer_ OCI image to [v1.8.20](https://github.com/kubernetes/autoscaler/releases/tag/addon-resizer-1.8.20). ([#1414](https://github.com/kubernetes-sigs/metrics-server/pull/1414)) [@stevehipwell](https://github.com/stevehipwell)

## [3.11.0] - 2023-08-03

### Added

- Added default _Metrics Server_ resource requests.

### Changed

- Updated the _Metrics Server_ OCI image to [v0.6.4](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.6.4).
- Updated the _addon-resizer_ OCI image to [v1.8.19](https://github.com/kubernetes/autoscaler/releases/tag/addon-resizer-1.8.19).

## [3.10.0] - 2023-04-12

### Added

- Added support for running under PodSecurity restricted.

### Fixed

- Fixed `auth-reader` role binding namespace to always use `kube-system`.
- Fixed addon-resizer configuration.
- Fixed container port default not having been updated to `10250`.

## [3.9.0] - 2023-03-28

### Added

- Added autoscaling support via the addon-resizer.

### Changed

- Updated the _Metrics Server_ OCI image to [v0.6.3](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.6.3).

### Fixed

- Fixed service labels/annotations.

## [3.8.4] - 2023-03-06

### Changed

- Changed the image registry location to `registry.k8s.io`.

## [3.8.3] - 2022-12-08

### Added

- Added support for topologySpreadConstraints.
- Always set resource namespaces explicitly.
- Allow configuring TLS on the APIService.
- Enabled service monitor relabelling.
- Added ability to set the scheduler name.
- Added support for common labels.

### Changed

- Updated the _Metrics Server_ OCI image to [v0.6.2](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.6.2).

## [3.8.2] - 2022-02-23

### Changed

- Changed chart to allow probes to be turned off completely (this is not advised unless you know what you're doing).

## [3.8.1] - 2022-02-09

### Changed

- Updated the _Metrics Server_ OCI image to [v0.6.1](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.6.1).

## [3.8.0] - 2022-02-08

### Added

- Added support for unauthenticated access to the /metrics endpoint.
- Added optional _Prometheus Operator_ `ServiceMonitor`.

### Changed

- Updated the _Metrics Server_ OCI image to [v0.6.0](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.6.0).

## [3.7.0] - 2021-11-18

### Changed

- Updated the _Metrics Server_ OCI image to [v0.5.2](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.5.2).

## [3.6.0] - 2021-10-18

### Added

- Added new `defaultArgs`` value to enable overriding the default arguments.

### Changed

- Updated the _Metrics Server_ OCI image to [v0.5.1](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.5.1).

## [3.5.0] - 2021-10-07

### Added

- Added initial Helm chart release from official repo.

<!--
RELEASE LINKS
-->
[UNRELEASED]: https://github.com/kubernetes-sigs/metrics-server/tree/master/charts/metrics-server
[3.12.1]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.12.1
[3.12.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.12.0
[3.11.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.11.0
[3.10.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.10.0
[3.9.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.9.0
[3.8.4]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.8.4
[3.8.3]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.8.3
[3.8.2]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.8.2
[3.8.1]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.8.1
[3.8.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.8.0
[3.7.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.7.0
[3.6.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.6.0
[3.5.0]: https://github.com/kubernetes-sigs/metrics-server/releases/tag/metrics-server-helm-chart-3.5.0
