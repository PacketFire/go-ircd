package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/PacketFire/go-ircd/parser"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
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
	ircd := NewIrcd(host)

	if err := ircd.Serve(l); err != nil {
		log.Printf("Serve: %s", err)
	}
}

type Ircd struct {
	hostname string

	clients map[string]*Client
	cm      sync.Mutex
}

func NewIrcd(host string) Ircd {
	return Ircd{hostname: host, clients: make(map[string]*Client)}
}

func (i *Ircd) NewClient(c net.Conn) Client {
	cl := Client{
		serv:    i,
		con:     c,
		inlines: bufio.NewScanner(c),
	}

	// grab just the ip of the remote user. pretty sure it's a TCPConn...
	tcpa := c.RemoteAddr().(*net.TCPAddr)

	cl.host = tcpa.IP.String()

	return cl
}

func (i *Ircd) AddClient(c *Client) error {
	if c.nick == "" {
		return fmt.Errorf("bad nick")
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	if _, ok := i.clients[c.nick]; ok {
		return fmt.Errorf("nick exists")
	}

	i.clients[c.nick] = c

	return nil
}

func (i *Ircd) RemoveClient(c *Client) error {
	if c.nick == "" {
		return fmt.Errorf("bad nick")
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	delete(i.clients, c.nick)

	return nil
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

func (i *Ircd) Privmsg(from *Client, to, msg string) {
	i.cm.Lock()
	defer i.cm.Unlock()

	if c, ok := i.clients[to]; ok {
		c.Privmsg(from.Prefix(), msg)
	} else {
		from.Send(i.hostname, "401", to, "No such user/nick")
	}
}

type Client struct {
	serv                 *Ircd
	con                  net.Conn
	inlines              *bufio.Scanner
	nick, user, realname string // various names
	host                 string
}

// make a prefix from this client
func (c *Client) Prefix() string {
	return fmt.Sprintf("%s!%s@%s", c.nick, c.user, c.host)
}

// client r/w
func (c *Client) Serve() {
	defer c.con.Close()

	for c.inlines.Scan() {
		var m parser.Message
		if err := m.UnmarshalText(c.inlines.Bytes()); err != nil {
			c.Error("malformed message")
		}

		log.Printf("Client.Serve: client says %s", m)

		switch m.Command {
		case "NICK":
			if len(m.Args) != 1 {
				c.Errorf("%s: bad arguments", m.Command)
			} else {
				c.nick = m.Args[0]
			}
		case "USER":
			if len(m.Args) != 4 {
				c.Errorf("%s: bad arguments", m.Command)
			} else {

				c.user = m.Args[0]
				c.realname = m.Args[3]
			}

			// just send motd after this
			c.DoMotd()
			c.serv.AddClient(c)
		case "PRIVMSG":
			if len(m.Args) != 2 {
				c.Errorf("%s: bad arguments", m.Command)
			} else {
				c.serv.Privmsg(c, m.Args[0], m.Args[1])
			}
		default:
			c.Errorf("not implemented: %s", m.Command)
		}

	}

	// dont forget to delete client when it's all over
	c.serv.RemoveClient(c)

	if err := c.inlines.Err(); err != nil {
		log.Printf("Client.Serve: %s", err)
	}
}

func (c *Client) Send(prefix, command string, args ...string) error {
	margs := []string{c.nick}
	margs = append(margs, args...)
	m := parser.Message{prefix, command, margs}

	b, err := m.MarshalText()
	if err != nil {
		log.Printf("marshalling %q failed: %s", err)
	}

	log.Printf("Send: %q", b)
	fmt.Fprintf(c.con, "%s\r\n", b)
	return err
}

func (c *Client) Privmsg(from, msg string) {
	c.Send(from, "PRIVMSG", msg)
}

func (c *Client) Error(content string) error {
	return c.Send(c.serv.hostname, "NOTICE", []string{"*", content}...)
}

func (c *Client) Errorf(format string, args ...interface{}) error {
	return c.Error(fmt.Sprintf(format, args...))
}

// do the dance to make the client think it connected
func (c *Client) DoMotd() {
	c.Send(c.serv.hostname, "001", fmt.Sprintf("Welcome %s", c.Prefix()))
	c.Send(c.serv.hostname, "002", fmt.Sprintf("We are %s running go-ircd", c.serv.hostname))
	c.Send(c.serv.hostname, "003", fmt.Sprintf("Created right now!"))
	c.Send(c.serv.hostname, "004", c.serv.hostname, "go-ircd", "v", "m")

	c.Send(c.serv.hostname, "251", fmt.Sprintf("There are %d users and %d services on %d servers", 1, 0, 1))
	c.Send(c.serv.hostname, "252", "0", "operator(s) online")
	c.Send(c.serv.hostname, "253", "0", "unknown connection(s)")
	c.Send(c.serv.hostname, "254", "0", "channel(s) formed")
	c.Send(c.serv.hostname, "255", fmt.Sprintf("I have %d clients and %d servers", 1, 1))

	c.Send(c.serv.hostname, "375", "- Message of the Day -")
	c.Send(c.serv.hostname, "372", "- Nada.")
	c.Send(c.serv.hostname, "376", "End of MOTD")
}
