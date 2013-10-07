package main

import (
  "flag"
  "math/rand"
  "time"
  "net"
  "log"
  "bufio"
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

  ircd := Ircd{}

  if err := ircd.Serve(l); err != nil {
    log.Printf("Serve: %s", err)
  }
}

type Ircd struct {

}

func (i *Ircd) NewClient(c net.Conn) Client {
  return Client{
    con: c,
    inlines: bufio.NewScanner(c),
  }
}

func (i *Ircd) Serve(l net.Listener) error {
  for {
    rw, err := l.Accept()
    if err != nil {
      return err
    }

    c := i.NewClient(rw)

    go c.Serve()
  }
}

type Client struct {
  con net.Conn
  inlines *bufio.Scanner
  nick, user, real string // various names
}

// client r/w
func (c *Client) Serve() {
  defer c.con.Close()

  for c.inlines.Scan() {
    log.Printf("Client.Serve: client says %q", c.inlines.Text())
  }

  if err := c.inlines.Err(); err != nil {
    log.Printf("Client.Serve: %s", err)
  }
}
