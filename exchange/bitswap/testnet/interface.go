package bitswap

import (
	bsnet "github.com/ipsn/go-ipfs/exchange/bitswap/network"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-testutil"
	peer "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peer"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
