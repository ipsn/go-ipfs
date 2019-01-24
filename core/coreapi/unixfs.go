package coreapi

import (
	"context"
	"fmt"

	"github.com/ipsn/go-ipfs/core"
	"github.com/ipsn/go-ipfs/filestore"

	coreiface "github.com/ipsn/go-ipfs/core/coreapi/interface"
	"github.com/ipsn/go-ipfs/core/coreapi/interface/options"
	"github.com/ipsn/go-ipfs/core/coreunix"

	ipld "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipld-format"
	mfs "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-mfs"
	bstore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-blockstore"
	ft "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs"
	uio "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/io"
	blockservice "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-blockservice"
	files "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-files"
	dag "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-merkledag"
	dagtest "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-merkledag/test"
	cidutil "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cidutil"
)

type UnixfsAPI CoreAPI

// Add builds a merkledag node from a reader, adds it to the blockstore,
// and returns the key representing that node.
func (api *UnixfsAPI) Add(ctx context.Context, files files.Node, opts ...options.UnixfsAddOption) (coreiface.ResolvedPath, error) {
	settings, prefix, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return nil, err
	}

	cfg, err := api.repo.Config()
	if err != nil {
		return nil, err
	}

	// check if repo will exceed storage limit if added
	// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
	// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
	//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
	//	res.SetError(err, cmdkit.ErrNormal)
	//	return
	//}

	if settings.NoCopy && !cfg.Experimental.FilestoreEnabled {
		return nil, filestore.ErrFilestoreNotEnabled
	}

	addblockstore := api.blockstore
	if !(settings.FsCache || settings.NoCopy) {
		addblockstore = bstore.NewGCBlockstore(api.baseBlocks, api.blockstore)
	}
	exch := api.exchange
	pinning := api.pinning

	if settings.OnlyHash {
		nilnode, err := core.NewNode(ctx, &core.BuildCfg{
			//TODO: need this to be true or all files
			// hashed will be stored in memory!
			NilRepo: true,
		})
		if err != nil {
			return nil, err
		}
		addblockstore = nilnode.Blockstore
		exch = nilnode.Exchange
		pinning = nilnode.Pinning
	}

	bserv := blockservice.New(addblockstore, exch) // hash security 001
	dserv := dag.NewDAGService(bserv)

	fileAdder, err := coreunix.NewAdder(ctx, pinning, addblockstore, dserv)
	if err != nil {
		return nil, err
	}

	fileAdder.Chunker = settings.Chunker
	if settings.Events != nil {
		fileAdder.Out = settings.Events
		fileAdder.Progress = settings.Progress
	}
	fileAdder.Hidden = settings.Hidden
	fileAdder.Wrap = settings.Wrap
	fileAdder.Pin = settings.Pin && !settings.OnlyHash
	fileAdder.Silent = settings.Silent
	fileAdder.RawLeaves = settings.RawLeaves
	fileAdder.NoCopy = settings.NoCopy
	fileAdder.Name = settings.StdinName
	fileAdder.CidBuilder = prefix

	switch settings.Layout {
	case options.BalancedLayout:
		// Default
	case options.TrickleLayout:
		fileAdder.Trickle = true
	default:
		return nil, fmt.Errorf("unknown layout: %d", settings.Layout)
	}

	if settings.Inline {
		fileAdder.CidBuilder = cidutil.InlineBuilder{
			Builder: fileAdder.CidBuilder,
			Limit:   settings.InlineLimit,
		}
	}

	if settings.OnlyHash {
		md := dagtest.Mock()
		emptyDirNode := ft.EmptyDirNode()
		// Use the same prefix for the "empty" MFS root as for the file adder.
		emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
		mr, err := mfs.NewRoot(ctx, md, emptyDirNode, nil)
		if err != nil {
			return nil, err
		}

		fileAdder.SetMfsRoot(mr)
	}

	nd, err := fileAdder.AddAllAndPin(files)
	if err != nil {
		return nil, err
	}
	return coreiface.IpfsPath(nd.Cid()), nil
}

func (api *UnixfsAPI) Get(ctx context.Context, p coreiface.Path) (files.Node, error) {
	ses := api.core().getSession(ctx)

	nd, err := ses.ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	return newUnixfsFile(ctx, ses.dag, nd)
}

// Ls returns the contents of an IPFS or IPNS object(s) at path p, with the format:
// `<link base58 hash> <link size in bytes> <link name>`
func (api *UnixfsAPI) Ls(ctx context.Context, p coreiface.Path) ([]*ipld.Link, error) {
	dagnode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	var ndlinks []*ipld.Link
	dir, err := uio.NewDirectoryFromNode(api.dag, dagnode)
	switch err {
	case nil:
		l, err := dir.Links(ctx)
		if err != nil {
			return nil, err
		}
		ndlinks = l
	case uio.ErrNotADir:
		ndlinks = dagnode.Links()
	default:
		return nil, err
	}

	links := make([]*ipld.Link, len(ndlinks))
	for i, l := range ndlinks {
		links[i] = &ipld.Link{Name: l.Name, Size: l.Size, Cid: l.Cid}
	}
	return links, nil
}

func (api *UnixfsAPI) core() *CoreAPI {
	return (*CoreAPI)(api)
}
