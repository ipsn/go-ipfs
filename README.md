# Ungx-ed fork of go-ipfs

[![GoDoc](https://godoc.org/github.com/ipsn/go-ipfs?status.svg)](https://godoc.org/github.com/ipsn/go-ipfs)

This repository is an unofficial fork of github.com/ipfs/go-ipfs, converted from a [`gx`](https://github.com/whyrusleeping/gx) based project to a plain Go project. The goal is to act as an IPFS library that can be imported and used from Go apps without the need of switching all dependency management over to `gx`. *As a bonus, this fork is [compatible with GoDoc](https://godoc.org/github.com/ipsn/go-ipfs)!*

For a rundown of why `gx` is not the best solution at the moment, please see the rationale section behind the [`ungx`](https://github.com/karalabe/ungx#why) project.

## Differences from upstream

Upstream [`go-ipfs`](github.com/ipfs/go-ipfs) is both a `gx` based project, as well as depends on many third party `gx` based packages. To use it in plain Go projects, all `gx` packages need to be resolved into their original canonical versions, or need to be converted into non-gx ones.

This fork uses the following logic to `ungx` go-ipfs:

 * If a dependency has a plain Go canonical version (e.g. `golang.org/x/net`), the dependency is converted from an IPFS multihash into its canonical path and vendored into the standard `vendor` folder. This ensures they play nice with the usual package managers.
 * If a dependency is *only* available as a `gx` project (e.g. `github.com/libp2p/go-libp2p`), the dependency is converted from an IPFS multihash into its canonical path, but is moved into the `gxlibs` folder within the main repository. This ensures external packages can import them.

Two caveats were also needed to enable this fork:

 * If multiple versions of the same plain Go dependency is found, these cannot be vendored in. In such cases, all clashing dependencies are embedded into the `gxlibs/gx/ipfs` folder with their original IPFS multihashes. This retains the original behavior whilst still permitting imports.
 * If an embedded dependency contains canonical path constraints (e.g. `golang.org/x/sys/unix`), these constraints are blindly deleted from the dependency sources. Unfortunately this is the only way to allow external code to import them without Go failing the build.

The ungx-ing process is done automatically for the `master` branch in a nightly Travis cron job from the [`ungx`](https://github.com/ipsn/go-ipfs/tree/ungx) branch in this repository. Upstream releases (i.e. tags) are not yet ungx-ed to prevent having to re-tag versions if a bug in `ungx` is discovered. Those will be added and tagged when the process is deemed reliable enough.

## Demo

The *hello-world* of IPFS is retrieving the official welcome page from the network. With the IPFS command line client this looks something like:

```
$ ipfs cat QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB
Hello and Welcome to IPFS!

██╗██████╗ ███████╗███████╗
██║██╔══██╗██╔════╝██╔════╝
██║██████╔╝█████╗  ███████╗
██║██╔═══╝ ██╔══╝  ╚════██║
██║██║     ██║     ███████║
╚═╝╚═╝     ╚═╝     ╚══════╝
[...]
```

Doing the same thing from Go is a bit more involved as it entails creating an ephemeral in-process IPFS node and using that as a gateway to retrieve the welcome page:

```go
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/core/coreunix"
)

func main() {
	node, err := core.NewNode(context.TODO(), &core.BuildCfg{Online: true})
	if err != nil {
		log.Fatalf("Failed to start IPFS node: %v", err)
	}
	reader, err := coreunix.Cat(context.TODO(), node, "QmPZ9gcCEpqKTo6aq61g2nXGUhM4iCL3ewB6LDXZCtioEB")
	if err != nil {
		log.Fatalf("Failed to look up IPFS welcome page: %v", err)
	}
	blob, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Fatalf("Failed to retrieve IPFS welcome page: %v", err)
	}
	fmt.Println(string(blob))
}
```

However, after the dependencies are met, our pure Go IPFS code works flawlessly:

```
$ go get -v github.com/ipsn/go-ipfs/core
$ go run ipfs.go
Hello and Welcome to IPFS!

██╗██████╗ ███████╗███████╗
██║██╔══██╗██╔════╝██╔════╝
██║██████╔╝█████╗  ███████╗
██║██╔═══╝ ██╔══╝  ╚════██║
██║██║     ██║     ███████║
╚═╝╚═╝     ╚═╝     ╚══════╝
[...]
```

### Proper dependencies

Although the above demo works correctly, running `go get -v github.com/ipsn/go-ipfs/core` is not for the faint of heart. It will place about 1193 packages into you `GOPATH` :scream:. A much better solution is to use your favorite dependency manager!

Demo with `govendor`:

```
$ go get -u github.com/kardianos/govendor
$ govendor init
$ govendor fetch -v +missing
$ go run ipfs.go
[...]
```

## Credits

This repository is maintained by Péter Szilágyi ([@karalabe](https://github.com/karalabe)), but authorship of all code contained inside belongs to the upstream [go-ipfs](https://github.com/ipfs/go-ipfs) project.

## License

[Same as upstream](https://github.com/ipfs/go-ipfs#license) (MIT).
