package coreapi

import (
	"context"
	"fmt"
	gopath "path"

	"github.com/ipsn/go-ipfs/core"

	coreiface "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/interface-go-ipfs-core"
	ipfspath "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-path"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-path/resolver"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	ipld "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipld-format"
	uio "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/io"
)

// ResolveNode resolves the path `p` using Unixfs resolver, gets and returns the
// resolved Node.
func (api *CoreAPI) ResolveNode(ctx context.Context, p coreiface.Path) (ipld.Node, error) {
	rp, err := api.ResolvePath(ctx, p)
	if err != nil {
		return nil, err
	}

	node, err := api.dag.Get(ctx, rp.Cid())
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ResolvePath resolves the path `p` using Unixfs resolver, returns the
// resolved path.
func (api *CoreAPI) ResolvePath(ctx context.Context, p coreiface.Path) (coreiface.ResolvedPath, error) {
	if _, ok := p.(coreiface.ResolvedPath); ok {
		return p.(coreiface.ResolvedPath), nil
	}

	ipath := ipfspath.Path(p.String())
	ipath, err := core.ResolveIPNS(ctx, api.namesys, ipath)
	if err == core.ErrNoNamesys {
		return nil, coreiface.ErrOffline
	} else if err != nil {
		return nil, err
	}

	var resolveOnce resolver.ResolveOnce

	switch ipath.Segments()[0] {
	case "ipfs":
		resolveOnce = uio.ResolveUnixfsOnce
	case "ipld":
		resolveOnce = resolver.ResolveSingle
	default:
		return nil, fmt.Errorf("unsupported path namespace: %s", p.Namespace())
	}

	r := &resolver.Resolver{
		DAG:         api.dag,
		ResolveOnce: resolveOnce,
	}

	node, rest, err := r.ResolveToLastNode(ctx, ipath)
	if err != nil {
		return nil, err
	}

	root, err := cid.Parse(ipath.Segments()[1])
	if err != nil {
		return nil, err
	}

	return coreiface.NewResolvedPath(ipath, node, root, gopath.Join(rest...)), nil
}
