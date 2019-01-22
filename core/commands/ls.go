package commands

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	cmdenv "github.com/ipsn/go-ipfs/core/commands/cmdenv"
	iface "github.com/ipsn/go-ipfs/core/coreapi/interface"

	unixfs "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs"
	uio "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/io"
	unixfspb "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/pb"
	cid "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	merkledag "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-merkledag"
	cmds "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmds"
	blockservice "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-blockservice"
	offline "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipld-format"
	cidenc "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cidutil/cidenc"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmdkit"
)

// LsLink contains printable data for a single ipld link in ls output
type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       unixfspb.Data_DataType
}

// LsObject is an element of LsOutput
// It can represent all or part of a directory
type LsObject struct {
	Hash  string
	Links []LsLink
}

// LsOutput is a set of printable data for directories,
// it can be complete or partial
type LsOutput struct {
	Objects []LsObject
}

const (
	lsHeadersOptionNameTime = "headers"
	lsResolveTypeOptionName = "resolve-type"
	lsSizeOptionName        = "size"
	lsStreamOptionName      = "stream"
)

var LsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path, with
the following format:

  <link base58 hash> <link size in bytes> <link name>

The JSON output contains type information.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(lsHeadersOptionNameTime, "v", "Print table headers (Hash, Size, Name)."),
		cmdkit.BoolOption(lsResolveTypeOptionName, "Resolve linked objects to find out their types.").WithDefault(true),
		cmdkit.BoolOption(lsSizeOptionName, "Resolve linked objects to find out their file size.").WithDefault(true),
		cmdkit.BoolOption(lsStreamOptionName, "s", "Enable exprimental streaming of directory entries as they are traversed."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		resolveType, _ := req.Options[lsResolveTypeOptionName].(bool)
		resolveSize, _ := req.Options[lsSizeOptionName].(bool)
		dserv := nd.DAG
		if !resolveType && !resolveSize {
			offlineexch := offline.Exchange(nd.Blockstore)
			bserv := blockservice.New(nd.Blockstore, offlineexch)
			dserv = merkledag.NewDAGService(bserv)
		}

		err = req.ParseBodyArgs()
		if err != nil {
			return err
		}
		paths := req.Arguments

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		var dagnodes []ipld.Node
		for _, fpath := range paths {
			p, err := iface.ParsePath(fpath)
			if err != nil {
				return err
			}
			dagnode, err := api.ResolveNode(req.Context, p)
			if err != nil {
				return err
			}
			dagnodes = append(dagnodes, dagnode)
		}
		ng := merkledag.NewSession(req.Context, nd.DAG)
		ro := merkledag.NewReadOnlyDagService(ng)

		stream, _ := req.Options[lsStreamOptionName].(bool)

		if !stream {
			output := make([]LsObject, len(req.Arguments))

			for i, dagnode := range dagnodes {
				dir, err := uio.NewDirectoryFromNode(ro, dagnode)
				if err != nil && err != uio.ErrNotADir {
					return fmt.Errorf("the data in %s (at %q) is not a UnixFS directory: %s", dagnode.Cid(), paths[i], err)
				}

				var links []*ipld.Link
				if dir == nil {
					links = dagnode.Links()
				} else {
					links, err = dir.Links(req.Context)
					if err != nil {
						return err
					}
				}
				outputLinks := make([]LsLink, len(links))
				for j, link := range links {
					lsLink, err := makeLsLink(req, dserv, resolveType, resolveSize, link, enc)
					if err != nil {
						return err
					}
					outputLinks[j] = *lsLink
				}
				output[i] = LsObject{
					Hash:  paths[i],
					Links: outputLinks,
				}
			}

			return cmds.EmitOnce(res, &LsOutput{output})
		}

		for i, dagnode := range dagnodes {
			dir, err := uio.NewDirectoryFromNode(ro, dagnode)
			if err != nil && err != uio.ErrNotADir {
				return fmt.Errorf("the data in %s (at %q) is not a UnixFS directory: %s", dagnode.Cid(), paths[i], err)
			}

			var linkResults <-chan unixfs.LinkResult
			if dir == nil {
				linkResults = makeDagNodeLinkResults(req, dagnode)
			} else {
				linkResults = dir.EnumLinksAsync(req.Context)
			}

			for linkResult := range linkResults {

				if linkResult.Err != nil {
					return linkResult.Err
				}
				link := linkResult.Link
				lsLink, err := makeLsLink(req, dserv, resolveType, resolveSize, link, enc)
				if err != nil {
					return err
				}
				output := []LsObject{{
					Hash:  paths[i],
					Links: []LsLink{*lsLink},
				}}
				if err = res.Emit(&LsOutput{output}); err != nil {
					return err
				}
			}
		}
		return nil
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			req := res.Request()
			lastObjectHash := ""

			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				out := v.(*LsOutput)
				lastObjectHash = tabularOutput(req, os.Stdout, out, lastObjectHash, false)
			}
		},
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *LsOutput) error {
			// when streaming over HTTP using a text encoder, we cannot render breaks
			// between directories because we don't know the hash of the last
			// directory encoder
			ignoreBreaks, _ := req.Options[lsStreamOptionName].(bool)
			tabularOutput(req, w, out, "", ignoreBreaks)
			return nil
		}),
	},
	Type: LsOutput{},
}

func makeDagNodeLinkResults(req *cmds.Request, dagnode ipld.Node) <-chan unixfs.LinkResult {
	links := dagnode.Links()
	linkResults := make(chan unixfs.LinkResult, len(links))
	defer close(linkResults)
	for _, l := range links {
		linkResults <- unixfs.LinkResult{
			Link: l,
			Err:  nil,
		}
	}
	return linkResults
}

func makeLsLink(req *cmds.Request, dserv ipld.DAGService, resolveType bool, resolveSize bool, link *ipld.Link, enc cidenc.Encoder) (*LsLink, error) {
	t := unixfspb.Data_DataType(-1)
	var size uint64

	switch link.Cid.Type() {
	case cid.Raw:
		// No need to check with raw leaves
		t = unixfs.TFile
		size = link.Size
	case cid.DagProtobuf:
		linkNode, err := link.GetNode(req.Context, dserv)
		if err == ipld.ErrNotFound && !resolveType && !resolveSize {
			// not an error
			linkNode = nil
		} else if err != nil {
			return nil, err
		}

		if pn, ok := linkNode.(*merkledag.ProtoNode); ok {
			d, err := unixfs.FSNodeFromBytes(pn.Data())
			if err != nil {
				return nil, err
			}
			if resolveType {
				t = d.Type()
			}
			if d.Type() == unixfs.TFile && resolveSize {
				size = d.FileSize()
			}
		}
	}
	return &LsLink{
		Name: link.Name,
		Hash: enc.Encode(link.Cid),
		Size: size,
		Type: t,
	}, nil
}

func tabularOutput(req *cmds.Request, w io.Writer, out *LsOutput, lastObjectHash string, ignoreBreaks bool) string {
	headers, _ := req.Options[lsHeadersOptionNameTime].(bool)
	stream, _ := req.Options[lsStreamOptionName].(bool)
	size, _ := req.Options[lsSizeOptionName].(bool)
	// in streaming mode we can't automatically align the tabs
	// so we take a best guess
	var minTabWidth int
	if stream {
		minTabWidth = 10
	} else {
		minTabWidth = 1
	}

	multipleFolders := len(req.Arguments) > 1

	tw := tabwriter.NewWriter(w, minTabWidth, 2, 1, ' ', 0)

	for _, object := range out.Objects {

		if !ignoreBreaks && object.Hash != lastObjectHash {
			if multipleFolders {
				if lastObjectHash != "" {
					fmt.Fprintln(tw)
				}
				fmt.Fprintf(tw, "%s:\n", object.Hash)
			}
			if headers {
				s := "Hash\tName"
				if size {
					s = "Hash\tSize\tName"
				}
				fmt.Fprintln(tw, s)
			}
			lastObjectHash = object.Hash
		}

		for _, link := range object.Links {
			s := "%[1]s\t%[3]s\n"

			switch {
			case link.Type == unixfs.TDirectory && size:
				s = "%[1]s\t-\t%[3]s/\n"
			case link.Type == unixfs.TDirectory && !size:
				s = "%[1]s\t%[3]s/\n"
			case size:
				s = "%s\t%v\t%s\n"
			}

			fmt.Fprintf(tw, s, link.Hash, link.Size, link.Name)
		}
	}
	tw.Flush()
	return lastObjectHash
}
