// +build solaris

package remote

import "github.com/ipsn/go-ipfs/gxlibs/golang.org/x/sys/unix"

func syscallDup(oldfd int, newfd int) (err error) {
	return unix.Dup2(oldfd, newfd)
}
