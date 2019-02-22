package pin

import (
	"context"
	"testing"
	"time"

	mdag "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-merkledag"
	bs "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-blockservice"

	util "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-util"
	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	ds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"
	dssync "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-exchange-offline"
)

var rand = util.NewTimeSeededRand()

func randNode() (*mdag.ProtoNode, cid.Cid) {
	nd := new(mdag.ProtoNode)
	nd.SetData(make([]byte, 32))
	rand.Read(nd.Data())
	k := nd.Cid()
	return nd, k
}

func assertPinned(t *testing.T, p Pinner, c cid.Cid, failmsg string) {
	_, pinned, err := p.IsPinned(c)
	if err != nil {
		t.Fatal(err)
	}

	if !pinned {
		t.Fatal(failmsg)
	}
}

func assertUnpinned(t *testing.T, p Pinner, c cid.Cid, failmsg string) {
	_, pinned, err := p.IsPinned(c)
	if err != nil {
		t.Fatal(err)
	}

	if pinned {
		t.Fatal(failmsg)
	}
}

func TestPinnerBasic(t *testing.T) {
	ctx := context.Background()

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv, dserv)

	a, ak := randNode()
	err := dserv.Add(ctx, a)
	if err != nil {
		t.Fatal(err)
	}

	// Pin A{}
	err = p.Pin(ctx, a, false)
	if err != nil {
		t.Fatal(err)
	}

	assertPinned(t, p, ak, "Failed to find key")

	// create new node c, to be indirectly pinned through b
	c, _ := randNode()
	err = dserv.Add(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	ck := c.Cid()

	// Create new node b, to be parent to a and c
	b, _ := randNode()
	err = b.AddNodeLink("child", a)
	if err != nil {
		t.Fatal(err)
	}

	err = b.AddNodeLink("otherchild", c)
	if err != nil {
		t.Fatal(err)
	}

	err = dserv.Add(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	bk := b.Cid()

	// recursively pin B{A,C}
	err = p.Pin(ctx, b, true)
	if err != nil {
		t.Fatal(err)
	}

	assertPinned(t, p, ck, "child of recursively pinned node not found")

	assertPinned(t, p, bk, "Recursively pinned node not found..")

	d, _ := randNode()
	d.AddNodeLink("a", a)
	d.AddNodeLink("c", c)

	e, _ := randNode()
	d.AddNodeLink("e", e)

	// Must be in dagserv for unpin to work
	err = dserv.Add(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	err = dserv.Add(ctx, d)
	if err != nil {
		t.Fatal(err)
	}

	// Add D{A,C,E}
	err = p.Pin(ctx, d, true)
	if err != nil {
		t.Fatal(err)
	}

	dk := d.Cid()
	assertPinned(t, p, dk, "pinned node not found.")

	// Test recursive unpin
	err = p.Unpin(ctx, dk, true)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Flush()
	if err != nil {
		t.Fatal(err)
	}

	np, err := LoadPinner(dstore, dserv, dserv)
	if err != nil {
		t.Fatal(err)
	}

	// Test directly pinned
	assertPinned(t, np, ak, "Could not find pinned node!")

	// Test recursively pinned
	assertPinned(t, np, bk, "could not find recursively pinned node")
}

func TestIsPinnedLookup(t *testing.T) {
	// We are going to test that lookups work in pins which share
	// the same branches. For that we will construct this tree:
	//
	// A5->A4->A3->A2->A1->A0
	//         /           /
	// B-------           /
	//  \                /
	//   C---------------
	//
	// We will ensure that IsPinned works for all objects both when they
	// are pinned and once they have been unpinned.
	aBranchLen := 6
	if aBranchLen < 3 {
		t.Fatal("set aBranchLen to at least 3")
	}

	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv, dserv)

	aNodes := make([]*mdag.ProtoNode, aBranchLen)
	aKeys := make([]cid.Cid, aBranchLen)
	for i := 0; i < aBranchLen; i++ {
		a, _ := randNode()
		if i >= 1 {
			err := a.AddNodeLink("child", aNodes[i-1])
			if err != nil {
				t.Fatal(err)
			}
		}

		err := dserv.Add(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		//t.Logf("a[%d] is %s", i, ak)
		aNodes[i] = a
		aKeys[i] = a.Cid()
	}

	// Pin A5 recursively
	if err := p.Pin(ctx, aNodes[aBranchLen-1], true); err != nil {
		t.Fatal(err)
	}

	// Create node B and add A3 as child
	b, _ := randNode()
	if err := b.AddNodeLink("mychild", aNodes[3]); err != nil {
		t.Fatal(err)
	}

	// Create C node
	c, _ := randNode()
	// Add A0 as child of C
	if err := c.AddNodeLink("child", aNodes[0]); err != nil {
		t.Fatal(err)
	}

	// Add C
	err := dserv.Add(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	ck := c.Cid()
	//t.Logf("C is %s", ck)

	// Add C to B and Add B
	if err := b.AddNodeLink("myotherchild", c); err != nil {
		t.Fatal(err)
	}
	err = dserv.Add(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	bk := b.Cid()
	//t.Logf("B is %s", bk)

	// Pin C recursively

	if err := p.Pin(ctx, c, true); err != nil {
		t.Fatal(err)
	}

	// Pin B recursively

	if err := p.Pin(ctx, b, true); err != nil {
		t.Fatal(err)
	}

	assertPinned(t, p, aKeys[0], "A0 should be pinned")
	assertPinned(t, p, aKeys[1], "A1 should be pinned")
	assertPinned(t, p, ck, "C should be pinned")
	assertPinned(t, p, bk, "B should be pinned")

	// Unpin A5 recursively
	if err := p.Unpin(ctx, aKeys[5], true); err != nil {
		t.Fatal(err)
	}

	assertPinned(t, p, aKeys[0], "A0 should still be pinned through B")
	assertUnpinned(t, p, aKeys[4], "A4 should be unpinned")

	// Unpin B recursively
	if err := p.Unpin(ctx, bk, true); err != nil {
		t.Fatal(err)
	}
	assertUnpinned(t, p, bk, "B should be unpinned")
	assertUnpinned(t, p, aKeys[1], "A1 should be unpinned")
	assertPinned(t, p, aKeys[0], "A0 should still be pinned through C")
}

func TestDuplicateSemantics(t *testing.T) {
	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)

	// TODO does pinner need to share datastore with blockservice?
	p := NewPinner(dstore, dserv, dserv)

	a, _ := randNode()
	err := dserv.Add(ctx, a)
	if err != nil {
		t.Fatal(err)
	}

	// pin is recursively
	err = p.Pin(ctx, a, true)
	if err != nil {
		t.Fatal(err)
	}

	// pinning directly should fail
	err = p.Pin(ctx, a, false)
	if err == nil {
		t.Fatal("expected direct pin to fail")
	}

	// pinning recursively again should succeed
	err = p.Pin(ctx, a, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFlush(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)
	p := NewPinner(dstore, dserv, dserv)
	_, k := randNode()

	p.PinWithMode(k, Recursive)
	if err := p.Flush(); err != nil {
		t.Fatal(err)
	}
	assertPinned(t, p, k, "expected key to still be pinned")
}

func TestPinRecursiveFail(t *testing.T) {
	ctx := context.Background()
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))
	dserv := mdag.NewDAGService(bserv)

	p := NewPinner(dstore, dserv, dserv)

	a, _ := randNode()
	b, _ := randNode()
	err := a.AddNodeLink("child", b)
	if err != nil {
		t.Fatal(err)
	}

	// NOTE: This isnt a time based test, we expect the pin to fail
	mctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()

	err = p.Pin(mctx, a, true)
	if err == nil {
		t.Fatal("should have failed to pin here")
	}

	err = dserv.Add(ctx, b)
	if err != nil {
		t.Fatal(err)
	}

	err = dserv.Add(ctx, a)
	if err != nil {
		t.Fatal(err)
	}

	// this one is time based... but shouldnt cause any issues
	mctx, cancel = context.WithTimeout(ctx, time.Second)
	defer cancel()
	err = p.Pin(mctx, a, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPinUpdate(t *testing.T) {
	ctx := context.Background()

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	bstore := blockstore.NewBlockstore(dstore)
	bserv := bs.New(bstore, offline.Exchange(bstore))

	dserv := mdag.NewDAGService(bserv)
	p := NewPinner(dstore, dserv, dserv)
	n1, c1 := randNode()
	n2, c2 := randNode()

	dserv.Add(ctx, n1)
	dserv.Add(ctx, n2)

	if err := p.Pin(ctx, n1, true); err != nil {
		t.Fatal(err)
	}

	if err := p.Update(ctx, c1, c2, true); err != nil {
		t.Fatal(err)
	}

	assertPinned(t, p, c2, "c2 should be pinned now")
	assertUnpinned(t, p, c1, "c1 should no longer be pinned")

	if err := p.Update(ctx, c2, c1, false); err != nil {
		t.Fatal(err)
	}

	assertPinned(t, p, c2, "c2 should be pinned still")
	assertPinned(t, p, c1, "c1 should be pinned now")
}
