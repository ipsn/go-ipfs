package decision

import (
	"fmt"
	"math"
	"testing"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-bitswap/wantlist"
	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	u "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-util"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peer"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-testutil"
)

// FWIW: At the time of this commit, including a timestamp in task increases
// time cost of Push by 3%.
func BenchmarkTaskQueuePush(b *testing.B) {
	q := newPRQ()
	peers := []peer.ID{
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
		testutil.RandPeerIDFatal(b),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := cid.NewCidV0(u.Hash([]byte(fmt.Sprint(i))))

		q.Push(peers[i%len(peers)], wantlist.Entry{Cid: c, Priority: math.MaxInt32})
	}
}
