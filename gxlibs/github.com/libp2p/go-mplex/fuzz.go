// +build gofuzz

package multiplex

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
)

func Fuzz(in []byte) int {
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	mplex := NewMultiplex(conn1, false)
	defer mplex.Close()
	go io.Copy(ioutil.Discard, conn2)
	go func() {
		for {
			s, err := mplex.Accept()
			if err != nil {
				return
			}
			go io.Copy(ioutil.Discard, s)
		}
	}()
	inBuf := bytes.NewBuffer(in)
	n, _ := io.Copy(conn2, inBuf)
	if n == int64(len(in)) {
		return 1
	} else {
		return 0
	}
}
