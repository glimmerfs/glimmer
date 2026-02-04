# Glimmer File System

This project is an implementation of the Glimmer File System,
a wire-compatible, cloud-native implementation of LustreFS.

## Motivation

The LustreFS project is widely used in very classic HPC
environments.
As HPC adopts Kubernetes and becomes more Cloud Native, users have
been trying to find better storage providers and usually had to select
significantly less performant options (like Ceph) in order to get
more reliability that their ephemeral workloads now require.

Cloud native storage has always been a traditionally hard
problem as Kubernetes is designed for stateless workloads.

This project aims to provide individual compatible components
from LustreFS as drop-in, wire-compatible replacements for some of
LustreFS's services that do not belong in kernel space.

This also aims to refresh the technology stack of LustreFS
significantly to encourage new developers to join and build
great new features.

## Early Roadmap

* [ ] LNet Client
* [ ] Glimmer Manager Service
* [ ] Glimmer Metadata Service
* [ ] Glimmer Operator

## Later Roadmap

* [ ] Glimmer Object Storage Service
* [ ] Rust-based FUSE object storage client
* [ ] Rust-based kernel object storage client
* [ ] Erasure Coding
* [ ] Hierarchical Storage Management

## License

This project has two main licenses:

* The default license of this project is Apache 2.0.
This is used for the Operator and Kubernetes APIs.
* The LNet interface and Glimmer Storage Services
are GPL 2.0.

Please refer to [COPYING](COPYING).

## Trademarks

Lustre and LustreFS are trademarked by the OpenSFS.
This project has no relationship with OpenSFS.
