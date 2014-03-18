package main

import (
	"bufio"
	"flag"
	"github.com/PacketFire/go-ircd/ircd"
	"github.com/davecheney/profile"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"
)

var (
	blockprofile = flag.Bool("blockprofile", false, "write block profile")
	cpuprofile   = flag.Bool("cpuprofile", false, "write cpu profile")
	httpdebug    = flag.Bool("httpdebug", false, "start net/http/pprof server on port 12345")
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

	if *httpdebug {
		go func() {
			log.Println(http.ListenAndServe(":12345", nil))
		}()
	}

	if *logfile != "" {
		lf, err := os.Create(*logfile)
		if err != nil {
			log.Fatal(err)
		}
		defer lf.Close()

		bf := bufio.NewWriter(lf)
		defer bf.Flush()

		log.SetOutput(bf)
	}

	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)

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
