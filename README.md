# Ungx-ed fork of go-ipfs

This repository is an unofficial fork of github.com/ipfs/go-ipfs, converted from a [`gx`](https://github.com/whyrusleeping/gx) based project to a plain Go project. The goal is to act as an IPFS library that can be imported and used from Go apps without the need of switching all dependency management over to `gx`.

For a rundown of why `gx` is not the best solution at the moment, please see the rationale section behind the [`ungx`](https://github.com/karalabe/ungx#why) project.

## Differences from upstream

Upstream [`go-ipfs`](github.com/ipfs/go-ipfs) is both a `gx` based project, as well as depends on many third party `gx` based packages. To use it in plain Go projects, all `gx` packages need to be resolved into their original canonical versions, or need to be converted into non-gx ones.

This fork uses the following logic to `ungx` go-ipfs:

 * If a dependency has a plain Go canonical version (e.g. `golang.org/x/net`), the dependency is converted from an IPFS multihash into its canonical path and vendored into the standard `vendor` folder. This ensures they play nice with the usual package managers.
 * If a dependency is *only* available as a `gx` project (e.g. `github.com/libp2p/go-libp2p`), the dependency is converted from an IPFS multihash into its canonical path, but is moved into the `gxlibs` folder within the main repository. This ensures external packages can import them.

The ungx-ing process is done automatically for the `master` branch in a nightly Travis cron job from the [`ungx`](https://github.com/ipsn/go-ipfs/tree/ungx) branch in this repository. Upstream releases (i.e. tags) are not yet ungx-ed to prevent having to re-tag versions if a bug in `ungx` is discovered. Those will be added and tagged when the process is deemed reliable enough.
