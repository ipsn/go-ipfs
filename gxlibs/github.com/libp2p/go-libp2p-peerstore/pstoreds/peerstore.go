package pstoreds

import (
	"context"
	"time"

	ds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"

	pstore "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peerstore"
	pstoremem "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peerstore/pstoremem"
)

// NewPeerstore creates a peerstore backed by the provided persistent datastore.
func NewPeerstore(ctx context.Context, ds ds.Batching) (pstore.Peerstore, error) {
	addrBook, err := NewAddrBook(ctx, ds, time.Second)
	if err != nil {
		return nil, err
	}

	ps := pstore.NewPeerstore(pstoremem.NewKeyBook(), addrBook, pstoremem.NewPeerMetadata())
	return ps, nil
}
