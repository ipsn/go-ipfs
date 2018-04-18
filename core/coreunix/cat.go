package coreunix

import (
	"context"

	core "github.com/ipsn/go-ipfs/core"
	path "github.com/ipsn/go-ipfs/path"
	resolver "github.com/ipsn/go-ipfs/path/resolver"
	uio "github.com/ipsn/go-ipfs/unixfs/io"
)

func Cat(ctx context.Context, n *core.IpfsNode, pstr string) (uio.DagReader, error) {
	r := &resolver.Resolver{
		DAG:         n.DAG,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	dagNode, err := core.Resolve(ctx, n.Namesys, r, path.Path(pstr))
	if err != nil {
		return nil, err
	}

	return uio.NewDagReader(ctx, dagNode, n.DAG)
}
