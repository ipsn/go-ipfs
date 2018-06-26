package bstest

import (
	. "github.com/ipsn/go-ipfs/blockservice"
	bitswap "github.com/ipsn/go-ipfs/exchange/bitswap"
	tn "github.com/ipsn/go-ipfs/exchange/bitswap/testnet"

	mockrouting "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-routing/mock"
	delay "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-delay"
)

// Mocks returns |n| connected mock Blockservices
func Mocks(n int) []BlockService {
	net := tn.VirtualNetwork(mockrouting.NewServer(), delay.Fixed(0))
	sg := bitswap.NewTestSessionGenerator(net)

	instances := sg.Instances(n)

	var servs []BlockService
	for _, i := range instances {
		servs = append(servs, New(i.Blockstore(), i.Exchange))
	}
	return servs
}
