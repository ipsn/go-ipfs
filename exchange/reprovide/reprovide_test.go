package reprovide_test

import (
	"context"
	"testing"

	blocks "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-block-format"
	mock "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-routing/mock"
	ds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"
	dssync "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore/sync"
	testutil "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-testutil"
	blockstore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-blockstore"
	pstore "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peerstore"

	. "github.com/ipsn/go-ipfs/exchange/reprovide"
)

func TestReprovide(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mrserv := mock.NewServer()

	idA := testutil.RandIdentityOrFatal(t)
	idB := testutil.RandIdentityOrFatal(t)

	clA := mrserv.Client(idA)
	clB := mrserv.Client(idB)

	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))

	blk := blocks.NewBlock([]byte("this is a test"))
	bstore.Put(blk)

	keyProvider := NewBlockstoreProvider(bstore)
	reprov := NewReprovider(ctx, clA, keyProvider)
	err := reprov.Reprovide()
	if err != nil {
		t.Fatal(err)
	}

	var providers []pstore.PeerInfo
	maxProvs := 100

	provChan := clB.FindProvidersAsync(ctx, blk.Cid(), maxProvs)
	for p := range provChan {
		providers = append(providers, p)
	}

	if len(providers) == 0 {
		t.Fatal("Should have gotten a provider")
	}

	if providers[0].ID != idA.ID() {
		t.Fatal("Somehow got the wrong peer back as a provider.")
	}
}
