package delayed

import (
	"testing"
	"time"

	datastore "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-datastore"
	delay "github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-delay"
)

func TestDelayed(t *testing.T) {
	d := New(datastore.NewMapDatastore(), delay.Fixed(time.Second))
	now := time.Now()
	k := datastore.NewKey("test")
	err := d.Put(k, "value")
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Get(k)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(now) < 2*time.Second {
		t.Fatal("There should have been a delay of 1 second in put and in get")
	}
}
