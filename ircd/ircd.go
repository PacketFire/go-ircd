package ircd

import (
	"bufio"
	"fmt"
	"github.com/PacketFire/go-ircd/parser"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Hostname string
	Listener net.Listener
}

type Ircd struct {
	conf     Config
	boottime time.Time

	// number of currently connected clients
	nclients int32

	// Client set, protected by RWMutex
	clients map[*Client]struct{}
	cm      sync.RWMutex

	channels map[string]*Channel
	chm      sync.RWMutex

	stop chan chan error
}

func New(conf Config) *Ircd {
	return &Ircd{
		conf:     conf,
		boottime: time.Now(),
		clients:  make(map[*Client]struct{}),
		channels: make(map[string]*Channel),
		stop:     make(chan chan error, 1),
	}
}

func (i *Ircd) Name() string {
	return i.conf.Hostname
}

func (i *Ircd) AddClient(c *Client) error {
	if c.nick == "" {
		return fmt.Errorf("bad nick")
	}

	if oldc := i.FindByNick(c.Str(CNick, "")); oldc != nil {
		return fmt.Errorf("nick exists")
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	i.clients[c] = struct{}{}
	atomic.AddInt32(&i.nclients, 1)

	return nil
}

func (i *Ircd) RemoveClient(c *Client) error {
	nick := c.Str(CNick, "")
	if nick == "" {
		return fmt.Errorf("bad nick")
	}

	if oldc := i.FindByNick(nick); oldc != nil {
		i.cm.Lock()
		defer i.cm.Unlock()

		delete(i.clients, oldc)
		atomic.AddInt32(&i.nclients, -1)
	} else {
		return fmt.Errorf("no such nick %s", nick)
	}

	return nil
}

func (i *Ircd) FindByNick(nick string) *Client {
	i.cm.RLock()
	defer i.cm.RUnlock()

	for c, _ := range i.clients {
		if c.Str(CNick, "") == nick {
			return c
		}
	}

	return nil
}

func (i *Ircd) FindChannel(name string) *Channel {
	i.chm.RLock()
	defer i.chm.RUnlock()

	return i.channels[name]
}

// Stop the irc server.
func (i *Ircd) Stop() error {
	stop := make(chan error)
	i.stop <- stop
	return <-stop
}

// Accept connections and serve clients until an error or a call to Stop occurs.
func (i *Ircd) Serve() (err error) {
	accept := make(chan net.Conn)

	go func() {
		for {
			rw, e := i.conf.Listener.Accept()
			if e != nil {
				break
			}
			accept <- rw
		}
	}()

forever:
	for {
		select {
		case stop := <-i.stop:
			i.conf.Listener.Close()
			stop <- err
			break forever
		case rw := <-accept:
			NewClient(i, rw)
		}
	}

	return
}

type MessageHandler func(i *Ircd, c *Client, m parser.Message) error

var (
	msgtab = map[string]MessageHandler{
		"NICK":    (*Ircd).HandleNick,
		"USER":    (*Ircd).HandleUser,
		"QUIT":    (*Ircd).HandleQuit,
		"PING":    (*Ircd).HandlePing,
		"PRIVMSG": (*Ircd).HandlePrivmsg,
		"MODE":    (*Ircd).HandleMode,
		"WHO":     (*Ircd).HandleWho,
		"JOIN":    (*Ircd).HandleJoin,
		"PART":    (*Ircd).HandlePart,
		"LIST":    (*Ircd).HandleList,
		"TOPIC":   (*Ircd).HandleTopic,
	}
)

func (i *Ircd) HandleNick(c *Client, m parser.Message) error {
	if len(m.Args) != 1 {
		c.EParams(m.Command)
		return nil
	} else if i.FindByNick(m.Args[0]) != nil {
		// check if nick is in use
		c.Send(i.Name(), "433", "*", m.Args[0], "Nickname already in use")
		return nil
	}

	oldnick := c.Str(CNick, (m.Args[0]))

	newn, newu := c.Str(CNick, ""), c.Str(CUser, "")
	if newn != "" && newu != "" {
		// ack the nick change only if we have an established user/nick
		c.Send(fmt.Sprintf("%s!%s@%s", oldnick, newu, c.Str(CHost, "")), "NICK", newn)

		// send motd when everything is ready, just once
		c.welcome.Do(func() {
			i.AddClient(c)
			i.DoMotd(c)
		})
	}

	return nil
}

func (i *Ircd) HandleUser(c *Client, m parser.Message) error {
	if len(m.Args) != 4 {
		c.EParams(m.Command)
		return nil
	}

	c.Str(CUser, m.Args[0])
	c.Str(CReal, m.Args[3])

	if c.Str(CNick, "") != "" && c.Str(CUser, "") != "" {
		// send motd when everything is ready, just once
		c.welcome.Do(func() {
			i.AddClient(c)
			i.DoMotd(c)
		})
	}

	return nil
}

// QUIT
func (i *Ircd) HandleQuit(c *Client, m parser.Message) error {
	var msg string

	if len(m.Args) == 0 {
		msg = ""
	} else {
		msg = m.Args[0]
	}

	c.quit.Do(func() {
		c.Quit <- fmt.Sprintf("QUIT: %s", msg)
	})

	return nil
}

func (i *Ircd) HandlePing(c *Client, m parser.Message) error {
	if len(m.Args) != 1 {
		c.EParams(m.Command)
		return nil
	}

	c.Send(i.Name(), "PONG", i.Name(), m.Args[0])

	return nil
}

func (i *Ircd) HandlePrivmsg(c *Client, m parser.Message) error {
	if len(m.Args) != 2 {
		c.EParams(m.Command)
		return nil
	}

	i.Privmsg(c, m.Args[0], m.Args[1])

	return nil
}

const (
	ModeQuery = iota
	ModeAdd
	ModeDel
)

// Change modes.
//
// TODO(mischief): check for user/chan modes when channels are implemented
func (i *Ircd) HandleMode(c *Client, m parser.Message) error {
	//dir := ModeAdd

	switch n := len(m.Args); n {
	case 1, 2:
		// query
		if strings.Index(m.Args[0], "#") == 0 || strings.Index(m.Args[0], "&") == 0 {
			// channel
			if ch := i.FindChannel(m.Args[0]); ch != nil {
				if n == 1 {
					// get
					ch.um.RLock()
					defer ch.um.RUnlock()
					return nil
				} else {
					// set
					ch.um.Lock()
					defer ch.um.Unlock()
					modes, _ := ch.modes.GetString()
					c.Send("324", m.Args[0], modes)
					return nil
				}
			} else {
				// not found
				c.Numeric("401", m.Args[0], "No such nick/channel")
				return nil
			}
		} else {
			// user
			if m.Args[0] != c.Nick("") {
				// do nothing if this query is not for the sending user
				return nil
			}

			if n == 1 {
				// get
				modes, _ := c.modes.GetString()
				c.Numeric("221", modes)
			} else {
				// set
				// TODO implement mode set for user
				c.EParams(m.Command)
				return nil
			}
		}
	default:
		c.EParams(m.Command)
		return nil
	}

	return nil
}

// WHO
//
// reply format:
// "<channel> <user> <host> <server> <nick> ( "H" / "G" > ["*"] [ ( "@" / "+" ) ] :<hopcount> <real name>" */
func (i *Ircd) HandleWho(c *Client, m parser.Message) error {
	//	var operOnly bool

	// TODO: zero args should show all non-invisible users
	if len(m.Args) < 1 {
		goto done
	}

	if strings.Index(m.Args[0], "#") == 0 || strings.Index(m.Args[0], "&") == 0 {
		// WHO for channel
		u := make(map[string]*Client)

		i.chm.RLock()
		ch, ok := i.channels[m.Args[0]]
		i.chm.RUnlock()

		if ok {
			ch.um.RLock()
			users := ch.users
			ch.um.RUnlock()

			for cl, _ := range users {
				u[cl.Str(CNick, "")] = cl
			}
		}

		for _, cl := range u {
			// read lock cl
			c.Send(i.Name(), "352", c.Str(CNick, ""), m.Args[0], cl.Str(CUser, ""), cl.Str(CHost, ""), i.Name(), cl.Str(CNick, ""), "H", fmt.Sprintf("0 %s", cl.Str(CReal, "")))
		}

	} else {
		// WHO for nick
		if cl := i.FindByNick(m.Args[0]); cl != nil {
			c.Send(i.Name(), "352", c.Str(CNick, ""), "0", cl.Str(CUser, ""), cl.Str(CHost, ""), i.Name(), cl.Str(CNick, ""), "H", fmt.Sprintf("0 %s", cl.Str(CReal, "")))
		}
	}

	/*
		if len(m.Args) == 2 && m.Args[1] == "o" {
			operOnly = true
		} else {
			operOnly = false
		}
	*/

done:
	c.Send(i.Name(), "315", "end of WHO")

	return nil
}

// JOIN
func (i *Ircd) HandleJoin(c *Client, m parser.Message) error {
	var thech *Channel
	if len(m.Args) < 1 {
		c.Numeric("461", m.Command, "need more parameters")
		return nil
	}

	log.Printf("%s attempts to join %q", c, m.Args[0])

	chans := strings.Split(m.Args[0], ",")

	i.chm.RLock()
	okchans := i.channels
	i.chm.RUnlock()

	for _, chname := range chans {
		if ch, ok := okchans[chname]; ok {
			thech = ch
		} else {
			// channel doesn't exist

			// sanity checks on channel.
			if chname == "" {
				c.Numeric("403", "*", "No such channel")
				return nil
			}

			if strings.Index(chname, "#") != 0 && strings.Index(chname, "&") != 0 {
				c.Numeric("403", chname, "No such channel")
				return nil
			}

			if len(chname) < 2 {
				c.Numeric("403", chname, "No such channel")
				return nil
			}

			// checks passed. make a channel.

			newch := NewChannel(i, chname)
			go newch.Run()

			i.chm.Lock()
			i.channels[chname] = newch
			i.chm.Unlock()

			thech = newch

			log.Printf("JOIN: New Channel %s", newch.name)
		}

		// add channel to client's list of joined channels
		c.mu.Lock()
		c.channels[chname] = thech
		c.mu.Unlock()

		// send messages about join
		thech.Join <- c

	}

	return nil
}

// PART
func (i *Ircd) HandlePart(c *Client, m parser.Message) error {
	if len(m.Args) < 1 {
		c.Numeric("461", m.Command, "need more parameters")
		return nil
	}

	/*
		var partmsg string

		if len(m.Args) > 1 {
			partmsg = m.Args[1]
		}
	*/

	chans := strings.Split(m.Args[0], ",")

	c.mu.Lock()
	onchans := c.channels
	c.mu.Unlock()

	for _, chname := range chans {
		if ch, ok := onchans[chname]; ok {
			ch.Part <- c
			//ch.Send(c.Prefix(), "PART", ch.name, partmsg)

			// remove this Channel from Client's map
			c.mu.Lock()
			delete(c.channels, chname)
			c.mu.Unlock()
		}
	}

	return nil
}

// LIST
func (i *Ircd) HandleList(c *Client, m parser.Message) error {
	i.chm.RLock()
	defer i.chm.RUnlock()

	if len(m.Args) < 1 {
		for _, ch := range i.channels {
			ch.um.RLock()
			c.Numeric("322", ch.name, fmt.Sprintf("%d", ch.nusers), ch.topic)
			ch.um.RUnlock()
		}
	} else {
		chans := strings.Split(m.Args[0], ",")

		for _, chname := range chans {
			if ch, ok := i.channels[chname]; ok {
				ch.um.RLock()
				c.Numeric("322", ch.name, fmt.Sprintf("%d", ch.nusers), ch.topic)
				ch.um.RUnlock()
			}
		}
	}

	c.Numeric("323", "end of LIST")

	return nil
}

// TOPIC
func (i *Ircd) HandleTopic(c *Client, m parser.Message) error {
	i.chm.Lock()
	defer i.chm.Unlock()

	if len(m.Args) < 1 {
		c.EParams(m.Command)
	}

	ch, ok := i.channels[m.Args[0]]

	if !ok {
		c.Numeric("403", "no such channel")
		return nil
	}

	switch len(m.Args) {
	case 1:
		// get topic
		if ch.topic == "" {
			c.Numeric("331", ch.name, "no topic")
		} else {
			c.Numeric("332", ch.name, ch.topic)
		}
	case 2:
		// set topic
		ch.topic = m.Args[1]
		ch.Send(c.Prefix(), "TOPIC", ch.name, ch.topic)
	}

	return nil
}

// Send a PRIVMSG from Client to channel or another client by name
func (i *Ircd) Privmsg(from *Client, to, msg string) {
	if tocl := i.FindByNick(to); tocl != nil {
		tocl.Privmsg(from, msg)
	} else if toch := i.FindChannel(to); toch != nil {
		toch.Privmsg(from, msg)
	} else {
		from.Send(i.Name(), "401", from.Nick(""), to, "No such user/nick")
	}
}

func (i *Ircd) DoMotd(c *Client) {
	c.Numeric("001", fmt.Sprintf("Welcome %s", c.Prefix()))
	c.Numeric("002", fmt.Sprintf("We are %s running go-ircd", i.Name()))
	c.Numeric("003", fmt.Sprintf("Booted %s", i.boottime))
	c.Numeric("004", i.Name(), "go-ircd", "v", "m")

	nc := atomic.LoadInt32(&i.nclients)

	c.Numeric("251", fmt.Sprintf("There are %d users and %d services on %d servers", nc, 0, 1))
	c.Numeric("252", "0", "operator(s) online")
	c.Numeric("253", "0", "unknown connection(s)")
	c.Numeric("254", "0", "channel(s) formed")
	c.Numeric("255", fmt.Sprintf("I have %d clients and %d servers", nc, 1))

	c.Numeric("375", "- Message of the Day -")
	c.Numeric("372", "- It works!")
	c.Numeric("376", "End of MOTD")

}

// Irc client.
type Client struct {
	// server reference
	serv *Ircd

	In   chan parser.Message
	Out  chan parser.Message
	Quit chan string

	// connection
	con net.Conn

	// scanner of incoming irc messages
	inlines *bufio.Scanner

	// used to prevent multiple clients appearing on NICK
	welcome sync.Once

	// only QUIT once
	quit sync.Once

	// RWMutex for the below data. we can't have multiple people reading/writing..
	mu sync.RWMutex

	// various names
	nick, user, realname string
	host                 string

	// user modes
	modes *Modeset

	// Channels we are on
	channels map[string]*Channel
}

// Allocate a new Client
func NewClient(ircd *Ircd, c net.Conn) *Client {
	cl := &Client{
		serv:     ircd,
		In:       make(chan parser.Message, 10),
		Out:      make(chan parser.Message, 100),
		Quit:     make(chan string),
		con:      c,
		inlines:  bufio.NewScanner(c),
		modes:    NewModeset(),
		channels: make(map[string]*Channel),
	}

	// grab just the ip of the remote user. pretty sure it's a TCPConn...
	tcpa := c.RemoteAddr().(*net.TCPAddr)
	cl.host = tcpa.IP.String()

	go cl.readin()
	go cl.loop()

	return cl
}

func (c *Client) readin() {
	defer c.con.Close()

	for c.inlines.Scan() {
		var m parser.Message
		if err := m.UnmarshalText(c.inlines.Bytes()); err != nil {
			c.Error("malformed message")
		}

		log.Printf("readin: %s -> %s", c, m)

		c.In <- m
	}

	c.serv.RemoveClient(c)

	if err := c.inlines.Err(); err != nil {
		c.quit.Do(func() {
			c.Quit <- fmt.Sprintf("ERROR: %s", err)
		})
		log.Printf("readin error: %s", err)
	} else {
		c.quit.Do(func() {
			c.Quit <- "QUIT: EOF"
		})
	}

	log.Printf("%s readin is done", c)
}

func (c *Client) loop() {

forever:
	for {
		select {
		case m := <-c.In:
			if h, ok := msgtab[m.Command]; ok {
				go func() {
					if err := h(c.serv, c, m); err != nil {
						c.Errorf("%s: %s", m.Command, err)
					}
				}()
			} else {
				c.Errorf("not implemented: %s", m.Command)
			}
		case m := <-c.Out:
			b, err := m.MarshalText()
			if err != nil {
				log.Printf("MarshalText: %s failed: %s", m, err)
			}

			log.Printf("Send: %s <- %s", c, m)
			fmt.Fprintf(c.con, "%s\r\n", b)
		case why := <-c.Quit:
			log.Printf("%s QUIT: %s", c.Prefix(), why)

			for _, ch := range c.channels {
				ch.Part <- c
				ch.Send(c.Prefix(), "QUIT", why)
			}

			break forever
		}
	}
}

const (
	CNick = iota
	CUser
	CReal
	CHost
)

func (c *Client) Str(op int, nw string) (old string) {
	c.mu.RLock()

	switch op {
	case CNick:
		old = c.nick
	case CUser:
		old = c.user
	case CReal:
		old = c.realname
	case CHost:
		old = c.host
	}

	if nw == "" {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	switch op {
	case CNick:
		c.nick = nw
	case CUser:
		c.user = nw
	case CReal:
		c.realname = nw
	case CHost:
		c.host = nw
	}

	return
}

// Set nick, return old. "" for get only.
func (c *Client) Nick(n string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n == "" {
		return c.nick
	}

	old := c.nick
	c.nick = n
	return old
}

// Set user, return old. "" for get only.
func (c *Client) User(u string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if u == "" {
		return c.user
	}

	old := c.user
	c.user = u
	return old
}

// Set realname, return old. "" for get only.
func (c *Client) Real(r string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if r == "" {
		return c.realname
	}

	old := c.realname
	c.realname = r
	return old
}

// Get host, read only.
func (c *Client) Host() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.host
}

// make a prefix from this client
func (c *Client) Prefix() string {
	return fmt.Sprintf("%s!%s@%s", c.nick, c.user, c.host)
}

func (c *Client) String() string {
	return c.Prefix()
}

func (c *Client) Send(prefix, command string, args ...string) {
	m := parser.Message{
		Prefix:  prefix,
		Command: command,
		Args:    args,
	}

	c.Out <- m
}

func (c *Client) Privmsg(from *Client, msg string) {
	c.Send(from.Prefix(), "PRIVMSG", c.nick, msg)
}

func (c *Client) Error(what string) {
	c.Send("", "ERROR", what)
}

func (c *Client) Errorf(format string, args ...interface{}) {
	c.Error(fmt.Sprintf(format, args...))
}

func (c *Client) EParams(cmd string) {
	c.Send(c.serv.Name(), "461", cmd, "not enough parameters")
}

func (c *Client) Numeric(code string, msg ...string) {
	c.Send(c.serv.Name(), code, append([]string{c.nick}, msg...)...)
}
