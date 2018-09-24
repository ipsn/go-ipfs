package datastore_test

import (
	"testing"

	dstore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"
	dstest "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore/test"
)

func TestMapDatastore(t *testing.T) {
	ds := dstore.NewMapDatastore()
	dstest.SubtestAll(t, ds)
}

func TestNullDatastore(t *testing.T) {
	ds := dstore.NewNullDatastore()
	// The only test that passes. Nothing should be found.
	dstest.SubtestNotFounds(t, ds)
}
