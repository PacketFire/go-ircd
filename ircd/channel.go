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

	Join      chan *Client
	Part      chan *Client
	Msg       chan parser.Message
	UsersList chan chan *Client
}

// Create a new channel
func NewChannel(serv *Ircd, name string) *Channel {
	ch := &Channel{
		serv:      serv,
		name:      name,
		users:     make(map[*Client]struct{}),
		modes:     NewModeset(),
		umodes:    make(map[*Client]Modeset),
		Join:      make(chan *Client, 100),
		Part:      make(chan *Client, 100),
		Msg:       make(chan parser.Message, 100),
		UsersList: make(chan chan *Client, 10),
	}
	return ch
}

func (ch *Channel) Run() {
	for {
		select {
		case c := <-ch.Join:
			if _, ok := ch.users[c]; ok {
				fmt.Errorf("user %s already in channel %s", c, ch.name)
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
			users := make([]string, len(ch.users))
			for u := range ch.users {
				users = append(users, u.nick)
			}
			c.Send(ch.serv.Name(), "353", "=", ch.name, strings.Join(users, " "))
			// end nicks
			c.Send(ch.serv.Name(), "366", ch.name, "end of /NAMES")

			// finally, inform other users of join
			for cl, _ := range ch.users {
				if cl.Str(CNick, "") != c.Str(CNick, "") {
					cl.Send(c.Prefix(), "JOIN", ch.name)
				}
			}

		case c := <-ch.Part:
			if _, ok := ch.users[c]; ok {
				for cl, _ := range ch.users {
					go cl.Send(c.Prefix(), "PART", ch.name, "part")
				}
				delete(ch.users, c)
				ch.nusers--
			} else {
				fmt.Errorf("%s not on channel %s", c, ch.name)
			}
		case m := <-ch.Msg:
			for cl, _ := range ch.users {
				if cl.Prefix() != m.Prefix {
					go cl.Send(m.Prefix, m.Command, append([]string{ch.name}, m.Args...)...)
				}
			}
		case out := <-ch.UsersList:
			// Request to list users on this channel
			for cl, _ := range ch.users {
				out <- cl
			}
			close(out)
		}
	}
}

// Generate a list of nicks on the channel
func (ch *Channel) Users() chan *Client {
	out := make(chan *Client, len(ch.users))

	ch.UsersList <- out

	return out
}

// Send an IRC Message to all users on a channel
func (ch *Channel) Send(prefix, command string, args ...string) error {
	ch.Msg <- parser.Message{Prefix: prefix, Command: command, Args: args}

	return nil
}

func (ch *Channel) Privmsg(from *Client, msg string) error {
	ch.Send(from.Prefix(), "PRIVMSG", msg)

	return nil
}
