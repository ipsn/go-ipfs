package coreunix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strconv"

	"github.com/ipsn/go-ipfs/pin"

	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-cid"
	bstore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-chunker"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-files"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-posinfo"
	ipld "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipld-format"
	logging "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-log"
	dag "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-merkledag"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-mfs"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/importer/balanced"
	ihelper "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-unixfs/importer/trickle"
	coreiface "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/interface-go-ipfs-core"
)

var log = logging.Logger("coreunix")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

var liveCacheSize = uint64(256 << 10)

type Link struct {
	Name, Hash string
	Size       uint64
}

// NewAdder Returns a new Adder used for a file add operation.
func NewAdder(ctx context.Context, p pin.Pinner, bs bstore.GCLocker, ds ipld.DAGService) (*Adder, error) {
	bufferedDS := ipld.NewBufferedDAG(ctx, ds)

	return &Adder{
		ctx:        ctx,
		pinning:    p,
		gcLocker:   bs,
		dagService: ds,
		bufferedDS: bufferedDS,
		Progress:   false,
		Hidden:     true,
		Pin:        true,
		Trickle:    false,
		Wrap:       false,
		Chunker:    "",
	}, nil
}

// Adder holds the switches passed to the `add` command.
type Adder struct {
	ctx        context.Context
	pinning    pin.Pinner
	gcLocker   bstore.GCLocker
	dagService ipld.DAGService
	bufferedDS *ipld.BufferedDAG
	Out        chan<- interface{}
	Progress   bool
	Hidden     bool
	Pin        bool
	Trickle    bool
	RawLeaves  bool
	Silent     bool
	Wrap       bool
	Name       string
	NoCopy     bool
	Chunker    string
	root       ipld.Node
	mroot      *mfs.Root
	unlocker   bstore.Unlocker
	tempRoot   cid.Cid
	CidBuilder cid.Builder
	liveNodes  uint64
}

func (adder *Adder) mfsRoot() (*mfs.Root, error) {
	if adder.mroot != nil {
		return adder.mroot, nil
	}
	rnode := unixfs.EmptyDirNode()
	rnode.SetCidBuilder(adder.CidBuilder)
	mr, err := mfs.NewRoot(adder.ctx, adder.dagService, rnode, nil)
	if err != nil {
		return nil, err
	}
	adder.mroot = mr
	return adder.mroot, nil
}

// SetMfsRoot sets `r` as the root for Adder.
func (adder *Adder) SetMfsRoot(r *mfs.Root) {
	adder.mroot = r
}

// Constructs a node from reader's data, and adds it. Doesn't pin.
func (adder *Adder) add(reader io.Reader) (ipld.Node, error) {
	chnk, err := chunker.FromString(reader, adder.Chunker)
	if err != nil {
		return nil, err
	}

	// Make sure all added nodes are written when done.
	defer adder.bufferedDS.Commit()

	params := ihelper.DagBuilderParams{
		Dagserv:    adder.bufferedDS,
		RawLeaves:  adder.RawLeaves,
		Maxlinks:   ihelper.DefaultLinksPerBlock,
		NoCopy:     adder.NoCopy,
		CidBuilder: adder.CidBuilder,
	}

	db, err := params.New(chnk)
	if err != nil {
		return nil, err
	}
	if adder.Trickle {
		return trickle.Layout(db)
	}

	return balanced.Layout(db)
}

// RootNode returns the root node of the Added.
func (adder *Adder) RootNode() (ipld.Node, error) {
	// for memoizing
	if adder.root != nil {
		return adder.root, nil
	}

	mr, err := adder.mfsRoot()
	if err != nil {
		return nil, err
	}
	root, err := mr.GetDirectory().GetNode()
	if err != nil {
		return nil, err
	}

	// if not wrapping, AND one root file, use that hash as root.
	if !adder.Wrap && len(root.Links()) == 1 {
		nd, err := root.Links()[0].GetNode(adder.ctx, adder.dagService)
		if err != nil {
			return nil, err
		}

		root = nd
	}

	adder.root = root
	return root, err
}

// Recursively pins the root node of Adder and
// writes the pin state to the backing datastore.
func (adder *Adder) PinRoot() error {
	root, err := adder.RootNode()
	if err != nil {
		return err
	}
	if !adder.Pin {
		return nil
	}

	rnk := root.Cid()

	err = adder.dagService.Add(adder.ctx, root)
	if err != nil {
		return err
	}

	if adder.tempRoot.Defined() {
		err := adder.pinning.Unpin(adder.ctx, adder.tempRoot, true)
		if err != nil {
			return err
		}
		adder.tempRoot = rnk
	}

	adder.pinning.PinWithMode(rnk, pin.Recursive)
	return adder.pinning.Flush()
}

// Finalize flushes the mfs root directory and returns the mfs root node.
func (adder *Adder) Finalize() (ipld.Node, error) {
	mr, err := adder.mfsRoot()
	if err != nil {
		return nil, err
	}
	var root mfs.FSNode
	rootdir := mr.GetDirectory()
	root = rootdir

	err = root.Flush()
	if err != nil {
		return nil, err
	}

	var name string
	if !adder.Wrap {
		children, err := rootdir.ListNames(adder.ctx)
		if err != nil {
			return nil, err
		}

		if len(children) == 0 {
			return nil, fmt.Errorf("expected at least one child dir, got none")
		}

		// Replace root with the first child
		name = children[0]
		root, err = rootdir.Child(name)
		if err != nil {
			return nil, err
		}
	}

	err = adder.outputDirs(name, root)
	if err != nil {
		return nil, err
	}

	err = mr.Close()
	if err != nil {
		return nil, err
	}

	return root.GetNode()
}

func (adder *Adder) outputDirs(path string, fsn mfs.FSNode) error {
	switch fsn := fsn.(type) {
	case *mfs.File:
		return nil
	case *mfs.Directory:
		names, err := fsn.ListNames(adder.ctx)
		if err != nil {
			return err
		}

		for _, name := range names {
			child, err := fsn.Child(name)
			if err != nil {
				return err
			}

			childpath := gopath.Join(path, name)
			err = adder.outputDirs(childpath, child)
			if err != nil {
				return err
			}

			fsn.Uncache(name)
		}
		nd, err := fsn.GetNode()
		if err != nil {
			return err
		}

		return outputDagnode(adder.Out, path, nd)
	default:
		return fmt.Errorf("unrecognized fsn type: %#v", fsn)
	}
}

func (adder *Adder) addNode(node ipld.Node, path string) error {
	// patch it into the root
	if path == "" {
		path = node.Cid().String()
	}

	if pi, ok := node.(*posinfo.FilestoreNode); ok {
		node = pi.Node
	}

	mr, err := adder.mfsRoot()
	if err != nil {
		return err
	}
	dir := gopath.Dir(path)
	if dir != "." {
		opts := mfs.MkdirOpts{
			Mkparents:  true,
			Flush:      false,
			CidBuilder: adder.CidBuilder,
		}
		if err := mfs.Mkdir(mr, dir, opts); err != nil {
			return err
		}
	}

	if err := mfs.PutNode(mr, path, node); err != nil {
		return err
	}

	if !adder.Silent {
		return outputDagnode(adder.Out, path, node)
	}
	return nil
}

// AddAllAndPin adds the given request's files and pin them.
func (adder *Adder) AddAllAndPin(file files.Node) (ipld.Node, error) {
	if adder.Pin {
		adder.unlocker = adder.gcLocker.PinLock()
	}
	defer func() {
		if adder.unlocker != nil {
			adder.unlocker.Unlock()
		}
	}()

	switch tf := file.(type) {
	case files.Directory:
		// Iterate over each top-level file and add individually. Otherwise the
		// single files.File f is treated as a directory, affecting hidden file
		// semantics.
		it := tf.Entries()
		for it.Next() {
			if err := adder.addFileNode(it.Name(), it.Node()); err != nil {
				return nil, err
			}
		}
		if it.Err() != nil {
			return nil, it.Err()
		}
		break
	default:
		if err := adder.addFileNode("", file); err != nil {
			return nil, err
		}
		break
	}

	// copy intermediary nodes from editor to our actual dagservice
	nd, err := adder.Finalize()
	if err != nil {
		return nil, err
	}

	if !adder.Pin {
		return nd, nil
	}
	return nd, adder.PinRoot()
}

func (adder *Adder) addFileNode(path string, file files.Node) error {
	defer file.Close()
	err := adder.maybePauseForGC()
	if err != nil {
		return err
	}

	if adder.liveNodes >= liveCacheSize {
		// TODO: A smarter cache that uses some sort of lru cache with an eviction handler
		mr, err := adder.mfsRoot()
		if err != nil {
			return err
		}
		if err := mr.FlushMemFree(adder.ctx); err != nil {
			return err
		}

		adder.liveNodes = 0
	}
	adder.liveNodes++

	switch f := file.(type) {
	case files.Directory:
		return adder.addDir(path, f)
	case *files.Symlink:
		return adder.addSymlink(path, f)
	case files.File:
		return adder.addFile(path, f)
	default:
		return errors.New("unknown file type")
	}
}

func (adder *Adder) addSymlink(path string, l *files.Symlink) error {
	sdata, err := unixfs.SymlinkData(l.Target)
	if err != nil {
		return err
	}

	dagnode := dag.NodeWithData(sdata)
	dagnode.SetCidBuilder(adder.CidBuilder)
	err = adder.dagService.Add(adder.ctx, dagnode)
	if err != nil {
		return err
	}

	return adder.addNode(dagnode, path)
}

func (adder *Adder) addFile(path string, file files.File) error {
	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if adder.Progress {
		rdr := &progressReader{file: reader, path: path, out: adder.Out}
		if fi, ok := file.(files.FileInfo); ok {
			reader = &progressReader2{rdr, fi}
		} else {
			reader = rdr
		}
	}

	dagnode, err := adder.add(reader)
	if err != nil {
		return err
	}

	addFileInfo, ok := file.(files.FileInfo)
	if ok {
		if addFileInfo.AbsPath() == os.Stdin.Name() && adder.Name != "" {
			path = adder.Name
			adder.Name = ""
		}
	}
	// patch it into the root
	return adder.addNode(dagnode, path)
}

func (adder *Adder) addDir(path string, dir files.Directory) error {
	log.Infof("adding directory: %s", path)

	mr, err := adder.mfsRoot()
	if err != nil {
		return err
	}
	err = mfs.Mkdir(mr, path, mfs.MkdirOpts{
		Mkparents:  true,
		Flush:      false,
		CidBuilder: adder.CidBuilder,
	})
	if err != nil {
		return err
	}

	it := dir.Entries()
	for it.Next() {
		fpath := gopath.Join(path, it.Name())

		// Skip hidden files when adding recursively, unless Hidden is enabled.
		if files.IsHidden(fpath, it.Node()) && !adder.Hidden {
			log.Infof("%s is hidden, skipping", fpath)
			continue
		}
		err = adder.addFileNode(fpath, it.Node())
		if err != nil {
			return err
		}
	}

	return it.Err()
}

func (adder *Adder) maybePauseForGC() error {
	if adder.unlocker != nil && adder.gcLocker.GCRequested() {
		err := adder.PinRoot()
		if err != nil {
			return err
		}

		adder.unlocker.Unlock()
		adder.unlocker = adder.gcLocker.PinLock()
	}
	return nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan<- interface{}, name string, dn ipld.Node) error {
	if out == nil {
		return nil
	}

	o, err := getOutput(dn)
	if err != nil {
		return err
	}

	out <- &coreiface.AddEvent{
		Path: o.Path,
		Name: name,
		Size: o.Size,
	}

	return nil
}

// from core/commands/object.go
func getOutput(dagnode ipld.Node) (*coreiface.AddEvent, error) {
	c := dagnode.Cid()
	s, err := dagnode.Size()
	if err != nil {
		return nil, err
	}

	output := &coreiface.AddEvent{
		Path: coreiface.IpfsPath(c),
		Size: strconv.FormatUint(s, 10),
	}

	return output, nil
}

type progressReader struct {
	file         io.Reader
	path         string
	out          chan<- interface{}
	bytes        int64
	lastProgress int64
}

func (i *progressReader) Read(p []byte) (int, error) {
	n, err := i.file.Read(p)

	i.bytes += int64(n)
	if i.bytes-i.lastProgress >= progressReaderIncrement || err == io.EOF {
		i.lastProgress = i.bytes
		i.out <- &coreiface.AddEvent{
			Name:  i.path,
			Bytes: i.bytes,
		}
	}

	return n, err
}

type progressReader2 struct {
	*progressReader
	files.FileInfo
}

func (i *progressReader2) Read(p []byte) (int, error) {
	return i.progressReader.Read(p)
}
