// +build freebsd

package util

import (
	unix "github.com/ipsn/go-ipfs/gxlibs/ipfs/QmPXvegq26x982cQjSfbTvSzZXn7GiaMwhhVPHkeTEhrPT/sys/unix"
)

func init() {
	supportsFDManagement = true
	getLimit = freebsdGetLimit
	setLimit = freebdsSetLimit
}

func freebsdGetLimit() (int64, int64, error) {
	rlimit := unix.Rlimit{}
	err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit)
	return rlimit.Cur, rlimit.Max, err
}

func freebdsSetLimit(soft int64, max int64) error {
	rlimit := unix.Rlimit{
		Cur: soft,
		Max: max,
	}
	return unix.Setrlimit(unix.RLIMIT_NOFILE, &rlimit)
}
