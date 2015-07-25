package ircd

import (
	"fmt"
	"log"
	"strings"

	"github.com/PacketFire/go-ircd/parser"
)

// NICK
func HandleNick(s Server, c Client, m parser.Message) error {
	if len(m.Args) != 1 {
		ErrorParams(s.Name(), c, m.Command)
		return nil
	} else if s.FindByNick(m.Args[0]) != nil {
		// check if nick is in use
		c.Send(s.Name(), "433", "*", m.Args[0], "nickname already in use")
		return nil
	}

	oldnick := c.SetNick(m.Args[0])

	newn, newu := c.Nick(), c.User()
	if newn != "" && newu != "" {
		// ack the nick change only if we have an established user/nick
		c.Send(fmt.Sprintf("%s!%s@%s", oldnick, newu, c.Host()), "NICK", newn)

		// send motd when everything is ready, just once
		ic := c.(*ircClient)
		ic.Greet()
	}

	return nil
}

// USER
func HandleUser(s Server, c Client, m parser.Message) error {
	if len(m.Args) != 4 {
		ErrorParams(s.Name(), c, m.Command)
		return nil
	}

	c.SetUser(m.Args[0])
	c.SetReal(m.Args[3])

	if c.Nick() != "" && c.User() != "" {
		// send motd when everything is ready, just once
		ic := c.(*ircClient)
		ic.Greet()
	}

	return nil
}

// QUIT
func HandleQuit(s Server, c Client, m parser.Message) error {
	var msg string

	if len(m.Args) == 0 {
		msg = ""
	} else {
		msg = m.Args[0]
	}

	msg = strings.TrimSpace(msg)

	ic := c.(*ircClient)
	ic.Quit(msg)

	return nil
}

// PING
func HandlePing(s Server, c Client, m parser.Message) error {
	if len(m.Args) != 1 {
		ErrorParams(s.Name(), c, m.Command)
		return nil
	}

	c.Send(s.Name(), "PONG", s.Name(), m.Args[0])

	return nil
}

// PRIVMSG
func HandlePrivmsg(s Server, c Client, m parser.Message) error {
	if len(m.Args) != 2 {
		ErrorParams(s.Name(), c, m.Command)
		return nil
	}

	return s.Privmsg(c, m.Args[0], m.Args[1])
}

const (
	ModeQuery = iota
	ModeAdd
	ModeDel
)

// Change modes.
//
// TODO(mischief): check for user/chan modes when channels are implemented
func HandleMode(s Server, c Client, m parser.Message) error {
	//dir := ModeAdd

	switch n := len(m.Args); n {
	case 1, 2:
		// query
		if strings.Index(m.Args[0], "#") == 0 || strings.Index(m.Args[0], "&") == 0 {
			// channel
			if ch := s.FindChannel(m.Args[0]); ch != nil {
				if n == 1 {
					// get
					return nil
				} else {
					// set
					//modes, _ := ch.modes.GetString()
					modes := ""
					c.Send("324", m.Args[0], modes)
					return nil
				}
			} else {
				// not found
				Numeric(s.Name(), c, "401", m.Args[0], "No such nick/channel")
				return nil
			}
		} else {
			// user
			if m.Args[0] != c.Nick() {
				// do nothing if this query is not for the sending user
				return nil
			}

			if n == 1 {
				// get
				//modes, _ := c.modes.GetString()
				modes := ""
				Numeric(s.Name(), c, "221", modes)
			} else {
				// set
				// TODO implement mode set for user
				ErrorParams(s.Name(), c, m.Command)
				return nil
			}
		}
	default:
		ErrorParams(s.Name(), c, m.Command)
	}

	return nil
}

// WHO
//
// reply format:
// "<channel> <user> <host> <server> <nick> ( "H" / "G" > ["*"] [ ( "@" / "+" ) ] :<hopcount> <real name>" */
func HandleWho(s Server, c Client, m parser.Message) error {
	//	var operOnly bool

	// TODO: zero args should show all non-invisible users
	if len(m.Args) < 1 {
		return c.Send(s.Name(), "315", "end of WHO")
	}

	cnick := c.Nick()

	if strings.Index(m.Args[0], "#") == 0 || strings.Index(m.Args[0], "&") == 0 {
		// WHO for channel
		u := make(map[string]Client)

		ch := s.FindChannel(m.Args[0])

		if ch != nil {
			for _, cl := range ch.Users() {
				u[cl.Nick()] = cl
			}
		}

		for _, cl := range u {
			// read lock cl
			c.Send(s.Name(), "352", cnick, m.Args[0], cl.User(), cl.Host(), s.Name(), cl.Nick(), "H", fmt.Sprintf("0 %s", cl.Real()))
		}

	} else {
		// WHO for nick
		if cl := s.FindByNick(m.Args[0]); cl != nil {
			c.Send(s.Name(), "352", cnick, "0", cl.User(), cl.Host(), s.Name(), cl.Nick(), "H", fmt.Sprintf("0 %s", cl.Real()))
		}
	}

	/*
		if len(m.Args) == 2 && m.Args[1] == "o" {
			operOnly = true
		} else {
			operOnly = false
		}
	*/

	return c.Send(s.Name(), "315", "end of WHO")
}

// JOIN
func HandleJoin(s Server, c Client, m parser.Message) error {
	var thech Channel
	if len(m.Args) < 1 {
		Numeric(s.Name(), c, "461", m.Command, "need more parameters")
		return nil
	}

	log.Printf("%s attempts to join %q", c, m.Args[0])

	chans := strings.Split(m.Args[0], ",")
	schans := s.Channels()
	for _, chname := range chans {
		if ch, ok := schans[chname]; ok {
			thech = ch
		} else {
			// channel doesn't exist

			// sanity checks on channel.
			if chname == "" {
				Numeric(s.Name(), c, "403", "*", "No such channel")
				return nil
			}

			if strings.Index(chname, "#") != 0 && strings.Index(chname, "&") != 0 {
				Numeric(s.Name(), c, "403", chname, "No such channel")
				return nil
			}

			if len(chname) < 2 {
				Numeric(s.Name(), c, "403", chname, "No such channel")
				return nil
			}

			// checks passed. make a channel.

			newch := NewChannel(s, s.Name(), chname)
			s.AddChannel(newch)

			thech = newch
			log.Printf("JOIN: New Channel %s", chname)
		}

		// add channel to client's list of joined channels
		c.Join(thech)

		// send messages about join
		thech.Join(c)
	}

	return nil
}

// PART
func HandlePart(s Server, c Client, m parser.Message) error {
	if len(m.Args) < 1 {
		Numeric(s.Name(), c, "461", m.Command, "need more parameters")
		return nil
	}

	/*
		var partmsg string

		if len(m.Args) > 1 {
			partmsg = m.Args[1]
		}
	*/

	chans := strings.Split(m.Args[0], ",")

	for _, chname := range chans {
		ch := s.FindChannel(chname)
		if ch != nil {
			if err := ch.Part(c); err == nil {
				//ch.Send(c.Prefix(), "PART", ch.name, partmsg)
				// remove this Channel from Client's map
				c.Part(ch)
			}
		}
	}

	return nil
}

// LIST
func HandleList(s Server, c Client, m parser.Message) error {
	schans := s.Channels()

	// rfc 2812 says this isn't needed but irssi seems to.
	Numeric(s.Name(), c, "321", "Channel", "Users  Name")

	if len(m.Args) < 1 {
		for _, ch := range schans {
			Numeric(s.Name(), c, "322", ch.Name(), fmt.Sprintf("%d", len(ch.Users())), ch.Topic())
		}
	} else {
		chans := strings.Split(m.Args[0], ",")

		for _, chname := range chans {
			if ch, ok := schans[chname]; ok {
				Numeric(s.Name(), c, "322", ch.Name(), fmt.Sprintf("%d", len(ch.Users())), ch.Topic())
			}
		}
	}

	Numeric(s.Name(), c, "323", "end of LIST")

	return nil
}

// TOPIC
func HandleTopic(s Server, c Client, m parser.Message) error {
	if len(m.Args) < 1 {
		ErrorParams(s.Name(), c, m.Command)
		return nil
	}

	ch := s.FindChannel(m.Args[0])
	if ch == nil {
		Numeric(s.Name(), c, "403", "no such channel")
		return nil
	}

	switch len(m.Args) {
	case 1:
		// get topic
		if ch.Topic() == "" {
			Numeric(s.Name(), c, "331", ch.Name(), "no topic")
		} else {
			Numeric(s.Name(), c, "332", ch.Name(), ch.Topic())
		}
	case 2:
		// set topic
		ch.SetTopic(m.Args[1])
		ch.Send(c.Prefix(), "TOPIC", ch.Name(), ch.Topic()+" ") // gross
	}

	return nil
}
