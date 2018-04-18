package namesys

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	path "github.com/ipsn/go-ipfs/path"

	dshelp "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-ds-help"
	testutil "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-testutil"
	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
	ds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"
	dssync "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore/sync"
	mockrouting "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-routing/mock"
	peer "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peer"
	ci "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-crypto"
)

type identity struct {
	testutil.PeerNetParams
}

func (p *identity) ID() peer.ID {
	return p.PeerNetParams.ID
}

func (p *identity) Address() ma.Multiaddr {
	return p.Addr
}

func (p *identity) PrivateKey() ci.PrivKey {
	return p.PrivKey
}

func (p *identity) PublicKey() ci.PubKey {
	return p.PubKey
}

func testNamekeyPublisher(t *testing.T, keyType int, expectedErr error, expectedExistence bool) {
	// Context
	ctx := context.Background()

	// Private key
	privKey, pubKey, err := ci.GenerateKeyPairWithReader(keyType, 2048, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// ID
	var id peer.ID
	switch keyType {
	case ci.Ed25519:
		id, err = peer.IDFromEd25519PublicKey(pubKey)
	default:
		id, err = peer.IDFromPublicKey(pubKey)
	}

	if err != nil {
		t.Fatal(err)
	}

	// Value
	value := path.Path("ipfs/TESTING")

	// Seqnum
	seqnum := uint64(0)

	// Eol
	eol := time.Now().Add(24 * time.Hour)

	// Routing value store
	p := testutil.PeerNetParams{
		ID:      id,
		PrivKey: privKey,
		PubKey:  pubKey,
		Addr:    testutil.ZeroLocalTCPAddress,
	}

	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	serv := mockrouting.NewServer()
	r := serv.ClientWithDatastore(context.Background(), &identity{p}, dstore)

	err = PutRecordToRouting(ctx, privKey, value, seqnum, eol, r, id)
	if err != nil {
		t.Fatal(err)
	}

	// Check for namekey existence in value store
	namekey, _ := IpnsKeysForID(id)
	_, err = r.GetValue(ctx, namekey)
	if err != expectedErr {
		t.Fatal(err)
	}

	// Also check datastore for completeness
	key := dshelp.NewKeyFromBinary([]byte(namekey))
	exists, err := dstore.Has(key)
	if err != nil {
		t.Fatal(err)
	}

	if exists != expectedExistence {
		t.Fatal("Unexpected key existence in datastore")
	}
}

func TestRSAPublisher(t *testing.T) {
	testNamekeyPublisher(t, ci.RSA, nil, true)
}

func TestEd22519Publisher(t *testing.T) {
	testNamekeyPublisher(t, ci.Ed25519, ds.ErrNotFound, false)
}
