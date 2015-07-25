package ircd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/PacketFire/go-ircd/parser"
)

type Channel interface {
	Name() string
	Join(c Client) error
	Part(c Client) error
	Close() error
	Users() []Client
	Topic() string
	SetTopic(topic string) string
	Send(prefix, command string, args ...string) error // send a message to the client
}

// An IRC Channel
type ircChannel struct {
	mu sync.Mutex

	// server instance
	srv     Server
	srvname string
	// Channel name, including #/&
	name string

	// Users joined to channel
	users map[string]Client
	// Count of current users
	nusers int

	// Channel topic
	topic string

	// Channel modes
	modes *Modeset

	// Modes of users on the channel
	umodes map[string]Modeset
}

// Create a new channel
func NewChannel(srv Server, srvname, name string) Channel {
	ch := &ircChannel{
		srv:     srv,
		srvname: srvname,
		name:    name,
		users:   make(map[string]Client),
		topic:   " ", // gross
		modes:   NewModeset(),
		umodes:  make(map[string]Modeset),
	}
	return ch
}

func (ch *ircChannel) Close() error {
	return nil
}

// Name returns the channel name, including the leading hash.
func (ch *ircChannel) Name() string {
	return ch.name
}

func (ch *ircChannel) Join(c Client) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if _, ok := ch.users[c.Nick()]; ok {
		return fmt.Errorf("user %s already in channel %s", c, ch.name)
	}

	ch.users[c.Nick()] = c
	ch.nusers++

	// inform user of join
	c.Send(c.Prefix(), "JOIN", ch.name)

	// topic
	if ch.topic != "" {
		c.Send(ch.srvname, "332", ch.name, ch.topic)
	}

	// time created?
	//who.Send(ch.serv.Name(), "333", who.nick, ch.name, "0")

	//ch.um.RLock()
	//defer ch.um.RUnlock()

	// nick list, meh
	users := make([]string, len(ch.users))
	for u := range ch.users {
		users = append(users, u)
	}

	Numeric(ch.srv.Name(), c, "353", "=", ch.name, strings.Join(users, " "))
	// end nicks
	Numeric(ch.srv.Name(), c, "366", ch.name, "end of /NAMES")

	// finally, inform other users of join
	cnick := c.Nick()
	cprefix := c.Prefix()
	for onick, ocl := range ch.users {
		if cnick != onick {
			ocl.Send(cprefix, "JOIN", ch.name)
		}
	}

	return nil
}

func (ch *ircChannel) Part(c Client) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	cnick := c.Nick()
	if _, ok := ch.users[cnick]; !ok {
		return fmt.Errorf("%s not on channel %s", c, ch.name)
	}

	cprefix := c.Prefix()
	for _, ocl := range ch.users {
		go ocl.Send(cprefix, "PART", ch.name, "part")
	}
	delete(ch.users, cnick)
	ch.nusers--

	return nil
}

// Generate a list of nicks on the channel
func (ch *ircChannel) Users() []Client {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	out := make([]Client, len(ch.users))
	i := 0

	for _, c := range ch.users {
		out[i] = c
		i++
	}

	return out
}

func (ch *ircChannel) Topic() string {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return ch.topic
}

func (ch *ircChannel) SetTopic(topic string) string {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	old := ch.topic
	ch.topic = topic
	return old
}

// Send an IRC Message to all users on a channel
func (ch *ircChannel) Send(prefix, command string, args ...string) error {
	m := parser.Message{Prefix: prefix, Command: command, Args: args}

	for onick, ocl := range ch.users {
		mnick, _, _ := SplitPrefix(m.Prefix)
		if mnick != onick {
			ocl.Send(m.Prefix, m.Command, m.Args...)
		}
	}

	return nil
}

func (ch *ircChannel) Privmsg(from Client, msg string) error {
	return ch.Send(from.Prefix(), "PRIVMSG", ch.Name(), msg)
}
