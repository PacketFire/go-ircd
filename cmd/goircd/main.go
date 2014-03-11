package main

import (
	"flag"
	"github.com/PacketFire/go-ircd/ircd"
	"log"
	"math/rand"
	"net"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	flag.Parse()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("main: recovered from %v", r)
		}
	}()

	l, err := net.Listen("tcp", ":6667")
	if err != nil {
		log.Fatalf("main: can't listen: %v", err)
	}

	host, err := os.Hostname()
	d := ircd.New(host)

	if err := d.Serve(l); err != nil {
		log.Printf("Serve: %s", err)
	}
}
