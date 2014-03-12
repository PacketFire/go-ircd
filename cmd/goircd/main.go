package main

import (
	"flag"
	"github.com/PacketFire/go-ircd/ircd"
	"github.com/davecheney/profile"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"time"
)

var (
	blockprofile = flag.Bool("blockprofile", false, "write block profile to block.prof")
	cpuprofile   = flag.Bool("cpuprofile", false, "write cpu profile to file")
	logfile      = flag.String("log", "", "log file")
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	flag.Parse()

	if *blockprofile {
		defer profile.Start(profile.BlockProfile).Stop()
	}

	if *cpuprofile {
		defer profile.Start(profile.CPUProfile).Stop()
	}

	if *logfile != "" {
		lf, err := os.Create(*logfile)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(lf)
	}

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

	conf := ircd.Config{
		Hostname: host,
		Listener: l,
	}

	d := ircd.New(conf)

	defer d.Stop()

	c := make(chan os.Signal, 1)
	i := make(chan error)
	signal.Notify(c, os.Interrupt, os.Kill)

	go func() {
		if err := d.Serve(); err != nil {
			i <- err
		}
		close(i)
	}()

	select {
	case e := <-i:
		log.Printf("Serve: %s", e)
	case sig := <-c:
		log.Printf("Recieved %s: shutting down", sig)
	}
}
