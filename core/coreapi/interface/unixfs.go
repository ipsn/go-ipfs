package iface

import (
	"context"

	"github.com/ipsn/go-ipfs/core/coreapi/interface/options"

	ipld "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipld-format"
	files "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-files"
)

type AddEvent struct {
	Name  string
	Path  ResolvedPath `json:",omitempty"`
	Bytes int64        `json:",omitempty"`
	Size  string       `json:",omitempty"`
}

// UnixfsAPI is the basic interface to immutable files in IPFS
// NOTE: This API is heavily WIP, things are guaranteed to break frequently
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	//
	// TODO: a long useful comment on how to use this for many different scenarios
	Add(context.Context, files.Node, ...options.UnixfsAddOption) (ResolvedPath, error)

	// Get returns a read-only handle to a file tree referenced by a path
	//
	// Note that some implementations of this API may apply the specified context
	// to operations performed on the returned file
	Get(context.Context, Path) (files.Node, error)

	// Ls returns the list of links in a directory
	Ls(context.Context, Path) ([]*ipld.Link, error)
}
