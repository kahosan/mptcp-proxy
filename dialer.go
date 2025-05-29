package main

import (
	"context"
	"fmt"
	"log"
	"net"
)

type targetedDailer struct {
	localDialer net.Dialer
	remoteAddr  string
	label       string
}

func newOutboundDialer(inputRemoteAddr string, inputLocalAddr string, label string) *targetedDailer {
	localAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", inputLocalAddr))
	if err != nil {
		log.Fatal(err)
	}
	td := &targetedDailer{
		localDialer: net.Dialer{
			LocalAddr: localAddr,
		},
		remoteAddr: inputRemoteAddr,
		label:      label,
	}
	return td
}

func (td *targetedDailer) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := td.localDialer.DialContext(ctx, "tcp", td.remoteAddr)
	if err != nil {
		return nil, err
	}
	log.Printf("Dialed to %v->%v", conn.LocalAddr(), td.remoteAddr)
	return conn, err
}

func (td *targetedDailer) Label() string {
	return fmt.Sprintf("%s|%s", td.label, td.remoteAddr)
}
