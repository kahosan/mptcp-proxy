package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/multipath"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	var server string
	var client string
	var remote string
	var localAddrs string

	flag.StringVar(&server, "s", "", "server mode, listen addr. e.g. 0.0.0.0:12345")
	flag.StringVar(&client, "c", "", "client mode, listen addr. e.g. 0.0.0.0:5001")
	flag.StringVar(&localAddrs, "a", "", "client mode, local addr. e.g. 192.168.0.10,192.168.0.11,192.168.0.12")
	flag.StringVar(&remote, "r", "", "server or client mode, proxy to remote server. e.g. 1.1.1.1:5201")
	flag.Parse()

	if len(client) > 0 {
		runClient(client, remote, strings.Split(localAddrs, ","))
		return
	}
	if len(server) > 0 {
		runServer(server, remote)
		return
	}
	flag.PrintDefaults()
}

func runClient(listen string, remote string, localAddrs []string) {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listen tcp at", listen)

	type Dial struct {
		cancel func()
		conn   net.Conn
	}
	preDialPool := make(chan Dial)
	go func() {
		for {
			ctx, cancel := context.WithCancel(context.Background())
			var ds []multipath.Dialer
			for i := range localAddrs {
				ds = append(ds, newOutboundDialer(remote, localAddrs[i], fmt.Sprintf("no.%d", i)))
			}
			remote, err := multipath.NewDialer("mptcp", ds).DialContext(ctx)
			if err != nil {
				log.Println(err)
				time.Sleep(time.Second)
				continue
			}
			preDialPool <- Dial{cancel: cancel, conn: remote}
		}
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("new conn", conn.RemoteAddr())
		go func() {
			dial := <-preDialPool
			log.Println("bicopy")
			biCopy(conn, dial.conn)
			dial.cancel()
		}()
	}
}

func runServer(listen string, remote string) {
	listeners := make([]net.Listener, 0)
	trackers := make([]multipath.StatsTracker, 0)

	l, err := net.Listen("tcp", listen)
	if err != nil {
		log.Fatal(err)
	}
	listeners = append(listeners, l)
	trackers = append(trackers, multipath.NullTracker{})
	ml := multipath.NewListener(listeners, trackers)

	log.Println("listen mptcp at", listen)

	preConnPool := make(chan net.Conn)
	go func() {
		for {
			remote, err := net.Dial("tcp", remote)
			if err != nil {
				log.Fatal(err)
			}
			preConnPool <- remote
		}
	}()

	for {
		conn, err := ml.Accept()
		if err != nil {
			log.Fatal(err)
		}
		log.Println("new conn", conn.RemoteAddr())
		go func() {
			remote := <-preConnPool
			biCopy(conn, remote)
		}()
	}
}

func biCopy(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		log.Println("copy")
		_, err := io.Copy(a, b)
		if err != nil {
			log.Println(err)
		}
		wg.Done()
		b.Close()
	}()
	go func() {
		log.Println("copy")
		_, err := io.Copy(b, a)
		if err != nil {
			log.Println(err)
		}
		wg.Done()
		a.Close()
	}()
	wg.Wait()
	log.Println("exit")
}
