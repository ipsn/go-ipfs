package mdutils

import (
	bsrv "github.com/ipsn/go-ipfs/blockservice"
	"github.com/ipsn/go-ipfs/exchange/offline"
	dag "github.com/ipsn/go-ipfs/merkledag"

	ds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"
	dssync "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-blockstore"
	ipld "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipld-format"
)

// Mock returns a new thread-safe, mock DAGService.
func Mock() ipld.DAGService {
	return dag.NewDAGService(Bserv())
}

// Bserv returns a new, thread-safe, mock BlockService.
func Bserv() bsrv.BlockService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	return bsrv.New(bstore, offline.Exchange(bstore))
}
