package main

import (
	"fmt"
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
	modes Modeset

	// Modes of users on the channel
	umodes map[*Client]Modeset
	// modes mutex
	mm sync.RWMutex
}

// Create a new channel
func NewChannel(serv *Ircd, name string) *Channel {
	return &Channel{name: name}
}

func (ch *Channel) AddClient(cl *Client) error {
	ch.um.Lock()
	defer ch.um.Unlock()

	cl.RLock()
	defer cl.RUnlock()
	if _, ok := ch.users[cl]; ok {
		return fmt.Errorf("user %s already in channel %s", cl.nick, ch.name)
	}

	ch.users[cl] = struct{}{}
	ch.nusers++

	return nil
}

func (ch *Channel) RemoveUser(cl *Client) error {
	ch.um.Lock()
	defer ch.um.Unlock()

	if _, ok := ch.users[cl]; ok {
		delete(ch.users, cl)
		ch.nusers--
	} else {
		return fmt.Errorf("%s not on channel %s", cl.nick, ch.name)
	}

	return nil
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

// Send a privmsg to the whole channel
func (ch *Channel) Privmsg(from *Client, msg string) error {
	ch.um.RLock()
	defer ch.um.RUnlock()

	for cl, _ := range ch.users {
		if cl.nick != from.nick {
			cl.Send(from.Prefix(), "PRIVMSG", ch.name, msg)
		}
	}

	return nil
}

// Inform joining client and other clients about the join
func (ch *Channel) Join(who *Client) error {
	ch.um.RLock()
	defer ch.um.RUnlock()

	// inform user of join
	who.Send(who.Prefix(), "JOIN", ch.name)

	// topic
	if ch.topic != "" {
		who.Send(ch.serv.hostname, "332", who.nick, ch.name, ch.topic)
	}

	// time created?
	//who.Send(ch.serv.hostname, "333", who.nick, ch.name, "0")

	// nick list, meh
	who.Send(ch.serv.hostname, "353", who.nick, "=", ch.name, strings.Join(ch.Users(), " "))
	// end nicks
	who.Send(ch.serv.hostname, "366", who.nick, ch.name, "end of /NAMES")

	// finally, inform other users of join
	for cl, _ := range ch.users {
		if cl.nick != who.nick {
			cl.Send(who.Prefix(), "JOIN", ch.name)
		}
	}

	return nil
}
