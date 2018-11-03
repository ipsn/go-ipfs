package main

import (
	plugin "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/iptb-plugins/browser"
	testbedi "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/iptb/testbed/interfaces"
)

var PluginName string
var NewNode testbedi.NewNodeFunc

func init() {
	PluginName = plugin.PluginName
	NewNode = plugin.NewNode
}
