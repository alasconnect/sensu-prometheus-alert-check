[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/alasconnect/sensu-prometheus-alert-check)
![Go Test](https://github.com/alasconnect/sensu-prometheus-alert-check/workflows/Go%20Test/badge.svg)
![goreleaser](https://github.com/alasconnect/sensu-prometheus-alert-check/workflows/goreleaser/badge.svg)

# prometheus-alert-check

## Table of Contents
- [prometheus-alert-check](#prometheus-alert-check)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Usage examples](#usage-examples)
    - [Asset registration](#asset-registration)
    - [Check definition](#check-definition)
  - [Installation from source](#installation-from-source)
  - [Contributing](#contributing)

## Overview

The prometheus-alert-check is a [Sensu Check][6] that monitors alerts in Prometheus.

## Usage examples

The command allows for the following arguments:

  - `-u, --url, PROMETHEUS_URL`: The base path of the Prometheus API. Default: **http://127.0.0.1:9090/**
  - `-i, --insecure-skip-verify, PROMETHEUS_SKIP_VERIFY`: Skip TLS certificate verification (not recommended!).
  - `-T, --trusted-ca-file, PROMETHEUS_CACERT`: TLS CA certificate bundle in PEM format.
  - `-t, --timeout, PROMETHEUS_TIMEOUT`: The number of seconds the test should wait for a response form the host (Default: 15).
  - `-f, --firing`: If specified, the check will only look for firing alerts.
  - `-p, --pending`: If specified, the check will only look for pending alerts.
  - `-l, --label`: Filter alerts by labels using a RegEx. Can be specified more than once. E.g. `--label myname=\"(Alert1|Alert2|^$)\"` will match 'Alert1', 'Alert2', and alerts with no label called 'myname'.
  - `-a, --annotation`: Filter alerts by annotations using a RegEx. Can be specified more than once.
  - `-w, --warning`: If specified, a failed check will result in a warning result.
  - `-c, --critical`: If specified, a failed check will result in a critical result. **Default**
  - `-F, --failure-level-label`: If specified, the result of a failed task will be determined by the specified Prometheus label. The label must have the value of 'warning' or 'critical'.
  - `-v, --verbose`: If specified, output will be more verbose.

### Asset registration

[Sensu Assets][10] are the best way to make use of this plugin. If you're not using an asset, please
consider doing so! If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the
following command to add the asset:

```
sensuctl asset add alasconnect/sensu-prometheus-alert-check
```

If you're using an earlier version of sensuctl, you can find the asset on the [Bonsai Asset Index][https://bonsai.sensu.io/assets/alasconnect/sensu-prometheus-alert-check].

### Check definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: sensu-prometheus-alert-check
  namespace: default
spec:
  command: sensu-prometheus-alert-check --url http://localhost:9090/
  subscriptions:
  - system
  runtime_assets:
  - alasconnect/sensu-prometheus-alert-check
```

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an Asset. If you would
like to compile and install the plugin from source or contribute to it, download the latest version
or create an executable script from this source.

From the local path of the sensu-prometheus-alert-check repository:

```
go build
```

## Contributing

For more information about contributing to this plugin, see [Contributing][1].

[1]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md
[2]: https://github.com/sensu/sensu-plugin-sdk
[3]: https://github.com/sensu-plugins/community/blob/master/PLUGIN_STYLEGUIDE.md
[4]: https://github.com/alasconnect/sensu-prometheus-alert-check/blob/master/.github/workflows/release.yml
[5]: https://github.com/alasconnect/sensu-prometheus-alert-check/actions
[6]: https://docs.sensu.io/sensu-go/latest/reference/checks/
[7]: https://github.com/sensu/check-plugin-template/blob/master/main.go
[8]: https://bonsai.sensu.io/
[9]: https://github.com/sensu/sensu-plugin-tool
[10]: https://docs.sensu.io/sensu-go/latest/reference/assets/
