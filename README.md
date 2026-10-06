# Gardener Extension for K8sstormcenter SOC



Project Gardener implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service.
Its main principle is to leverage Kubernetes concepts for all of its tasks.

Recently, most of the vendor specific logic has been developed [in-tree](https://github.com/gardener/gardener).
However, the project has grown to a size where it is very hard to extend, maintain, and test.
With [GEP-1](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md) we have proposed how the architecture can be changed in a way to support external controllers that contain their very own vendor specifics. This way, we can keep Gardener core clean and independent.

This extension integrates Kubescape, Pixie and K8sstormcenter, the `soc suite`, into Gardener shoot clusters. 



## Overview
- **Extension Name:** `gardener-extension-shoot-rtsecurity-service`
- **Purpose:** Deploy and manage the full runtime security (and soon also forensics) in shoot clusters via Gardener’s extension mechanism
- **Features:**
  - Automated deployemnt with lifecycle management
  - Deployment with standard or custom detection configs
  - Support for clickhouse event storage (either local or remote)


## Getting Started

### Prerequisites
- A running Gardener landscape (see Gardener documentation)
- Access to a shoot cluster
- Extension enabled in the landscape configuration via [extension configuration](docs/extension-configuration.md)

### Installation
Add the extension to your shoot manifest:
```yaml
  extensions:
    - type: shoot-rtsecurity-service
```

For a full shoot extension section configuration, refer to the [configuration documentation](docs/extension-configuration.md)



## Learn more!

Please find further resources about our project here:

* [Our landing page gardener.cloud](https://gardener.cloud/)
* ["Gardener, the Kubernetes Botanist" blog on kubernetes.io](https://kubernetes.io/blog/2018/05/17/gardener/)
* ["Gardener Project Update" blog on kubernetes.io](https://kubernetes.io/blog/2019/12/02/gardener-project-update/)
* [Gardener Extensions Golang library](https://godoc.org/github.com/gardener/gardener/extensions/pkg)
* [GEP-1 (Gardener Enhancement Proposal) on extensibility](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md)
* [Extensibility API documentation](https://github.com/gardener/gardener/tree/master/docs/extensions)

<p align="center"><img alt="Bundesministerium für Wirtschaft und Energie (BMWE)-EU funding logo" src="https://apeirora.eu/assets/img/BMWK-EU.png" width="400"/></p>
