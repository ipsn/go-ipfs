package cmds

import (
	"github.com/ipsn/go-ipfs/gxlibs/github.com/ipfs/go-ipfs-cmdkit"
)

// Response is the result of a command request. Response is returned to the client.
type Response interface {
	Request() *Request

	Error() *cmdkit.Error
	Length() uint64

	// Next returns the next emitted value.
	// The returned error can be a network or decoding error.
	Next() (interface{}, error)
}
