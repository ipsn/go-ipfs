package utils

import (
	"context"
	"io"
	"testing"
	"time"

	peer "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-peer"
	tpt "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-transport"
	smux "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-stream-muxer"
	ma "github.com/ipsn/go-ipfs/gxlibs/github.com/multiformats/go-multiaddr"
)

type streamAndConn struct {
	stream smux.Stream
	conn   tpt.Conn
}

var testData = []byte("this is some test data")

var Subtests = map[string]func(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID){
	"Protocols": SubtestProtocols,
	"Basic":     SubtestBasic,
	"Cancel":    SubtestCancel,
}

func SubtestTransport(t *testing.T, ta, tb tpt.Transport, addr string, peerA peer.ID) {
	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	for n, f := range Subtests {
		t.Run(n, func(t *testing.T) {
			f(t, ta, tb, maddr, peerA)
		})
	}
}

func SubtestProtocols(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	rawIPAddr, _ := ma.NewMultiaddr("/ip4/1.2.3.4")
	if ta.CanDial(rawIPAddr) || tb.CanDial(rawIPAddr) {
		t.Error("nothing should be able to dial raw IP")
	}

	tprotos := make(map[int]bool)
	for _, p := range ta.Protocols() {
		tprotos[p] = true
	}

	if !ta.Proxy() {
		protos := maddr.Protocols()
		proto := protos[len(protos)-1]
		if !tprotos[proto.Code] {
			t.Errorf("transport should have reported that it supports protocol '%s' (%d)", proto.Name, proto.Code)
		}
	} else {
		found := false
		for _, proto := range maddr.Protocols() {
			if tprotos[proto.Code] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("didn't find any matching proxy protocols in maddr: %s", maddr)
		}
	}
}

func SubtestBasic(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	list, err := ta.Listen(maddr)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := list.Accept()
		if err != nil {
			t.Fatal(err)
			return
		}
		s, err := c.AcceptStream()
		if err != nil {
			c.Close()
			t.Fatal(err)
			return
		}

		buf := make([]byte, len(testData))
		_, err = io.ReadFull(s, buf)
		if err != nil {
			t.Fatal(err)
			return
		}

		n, err := s.Write(testData)
		if err != nil {
			t.Fatal(err)
			return
		}
		s.Close()

		if n != len(testData) {
			t.Fatal(err)
			return
		}
	}()

	if !tb.CanDial(list.Multiaddr()) {
		t.Error("CanDial should have returned true")
	}

	c, err := tb.Dial(ctx, list.Multiaddr(), peerA)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	s, err := c.OpenStream()
	if err != nil {
		t.Fatal(err)
	}

	n, err := s.Write(testData)
	if err != nil {
		t.Fatal(err)
		return
	}

	if n != len(testData) {
		t.Fatalf("failed to write enough data (a->b)")
		return
	}

	buf := make([]byte, len(testData))
	_, err = io.ReadFull(s, buf)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func SubtestCancel(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	list, err := ta.Listen(maddr)
	if err != nil {
		t.Fatal(err)
	}
	defer list.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := tb.Dial(ctx, list.Multiaddr(), peerA)
		if err == nil {
			c.Close()
			t.Fatal("dial should have failed")
		}
	}()

	time.Sleep(time.Millisecond)
	cancel()
	<-done

	done = make(chan struct{})
	go func() {
		defer close(done)
		c, err := list.Accept()
		if err == nil {
			c.Close()
			t.Fatal("accept should have failed")
		}
	}()
	time.Sleep(time.Millisecond)
	list.Close()
	<-done
}
