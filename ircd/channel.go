package ircd

import (
	"fmt"
	"github.com/PacketFire/go-ircd/parser"
	"strings"
	"sync"
)

// An IRC Channel
type Channel struct {
	// server instance
	serv *Ircd
	// Channel name, including #/&
	name string

	// Users joined to channel
	users map[*Client]struct{}
	// Count of current users
	nusers int
	// users mutex
	um sync.RWMutex

	// Channel topic
	topic string

	// Channel modes
	modes *Modeset

	// Modes of users on the channel
	umodes map[*Client]Modeset

	Join chan *Client
	Part chan *Client
	Msg  chan parser.Message
}

// Create a new channel
func NewChannel(serv *Ircd, name string) *Channel {
	ch := &Channel{
		serv:   serv,
		name:   name,
		users:  make(map[*Client]struct{}),
		modes:  NewModeset(),
		umodes: make(map[*Client]Modeset),
		Join:   make(chan *Client, 5),
		Part:   make(chan *Client, 5),
		Msg:    make(chan parser.Message, 5),
	}
	return ch
}

func (ch *Channel) Run() {
	for {
		select {
		case c := <-ch.Join:
			if _, ok := ch.users[c]; ok {
				fmt.Errorf("user %s already in channel %s", c, ch.name)
				// some reply about being on channel
				break
			}
			ch.users[c] = struct{}{}
			ch.nusers++
			// inform user of join
			c.Send(c.Prefix(), "JOIN", ch.name)

			// topic
			if ch.topic != "" {
				c.Send(ch.serv.Name(), "332", ch.name, ch.topic)
			}

			// time created?
			//who.Send(ch.serv.Name(), "333", who.nick, ch.name, "0")

			//ch.um.RLock()
			//defer ch.um.RUnlock()

			// nick list, meh
			c.Send(ch.serv.Name(), "353", "=", ch.name, strings.Join(ch.Users(), " "))
			// end nicks
			c.Send(ch.serv.Name(), "366", ch.name, "end of /NAMES")

			// finally, inform other users of join
			for cl, _ := range ch.users {
				if cl.nick != c.nick {
					cl.Send(c.Prefix(), "JOIN", ch.name)
				}
			}

		case c := <-ch.Part:
			if _, ok := ch.users[c]; ok {
				delete(ch.users, c)
				ch.nusers--
			} else {
				fmt.Errorf("%s not on channel %s", c, ch.name)
			}
		case m := <-ch.Msg:
			for cl, _ := range ch.users {
				if cl.Prefix() != m.Prefix {
					cl.Send(m.Prefix, m.Command, append([]string{ch.name}, m.Args...)...)
				}
			}
		}
	}
}

// Generate a slice of nicks on the channel
func (ch *Channel) Users() []string {
	var users []string

	ch.um.RLock()
	defer ch.um.RUnlock()

	for cl, _ := range ch.users {
		users = append(users, cl.nick)
	}

	return users
}

// Send an IRC Message to all users on a channel
func (ch *Channel) Send(prefix, command string, args ...string) error {
	ch.Msg <- parser.Message{prefix, command, args}

	return nil
}

func (ch *Channel) Privmsg(from *Client, msg string) error {
	ch.Msg <- parser.Message{from.Prefix(), "PRIVMSG", []string{msg}}

	return nil
}
