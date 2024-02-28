# Cluster Stack Provider OpenStack

[![GitHub Latest Release](https://img.shields.io/github/v/release/SovereignCloudStack/cluster-stack-provider-openstack?logo=github)](https://github.com/SovereignCloudStack/cluster-stack-provider-openstack/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/sovereignCloudStack/cluster-stack-provider-openstack)](https://goreportcard.com/report/github.com/sovereignCloudStack/cluster-stack-provider-openstack)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Overview

Refer to the [overview page](./docs/overview.md) in the `docs` directory.

## Quickstart

Refer to the [quickstart page](./docs/quickstart.md) in the `docs` directory.

## Developer Guide

Refer to the [developer guide page](./docs/develop.md) to find more information about how to develop this operator.

## Documentation

Explore the documentation stored in the [docs](./docs) directory or view the rendered version online at <https://docs.scs.community/docs/category/operating-scs>.

## Compatibility with Cluster Stack Operator

|                         | CSO `v0.1.0-alpha.2` | CSO `v0.1.0-alpha.3` |
| ----------------------- | -------------------- | -------------------- |
| CSPO `v0.1.0-alpha.rc1` | ✓ |   |
| CSPO `v0.1.0-alpha.1`   | ✓ | ✓ |
| CSPO `v0.1.0-alpha.2`   | ✓ | ✓ |

## Controllers

CSPO consists of two controllers. They should ensure that the desired node images are present in the targeted OpenStack project.
Refer to the documentation for the CSPO [controllers](./docs/controllers.md) in the `docs` directory.

## API Reference

CSPO currently exposes the following APIs:

- the CSPO Custom Resource Definitions (CRDs): [documentation](https://doc.crds.dev/github.com/SovereignCloudStack/cluster-stack-provider-openstack)
- Golang APIs: tbd
