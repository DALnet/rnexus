package irc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dalnet/rnexus/internal/routing"
	"github.com/dalnet/rnexus/internal/storage"
)

// handleCommand processes a command from a verified IRC operator
func (c *Client) handleCommand(nick, hostmask, message string) {
	message = strings.TrimSpace(message)
	cmd := strings.ToLower(strings.Fields(message)[0])

	switch {
	case cmd == "!help":
		c.cmdHelp(nick, hostmask, message)
	case cmd == "!links":
		c.cmdLinks(nick, hostmask, message, false)
	case cmd == "!summary":
		c.cmdLinks(nick, hostmask, message, true)
	case cmd == "!map":
		c.cmdMap(nick, hostmask, message)
	case cmd == "!uplinks":
		c.cmdUplinks(nick, hostmask, message)
	case cmd == "!logs":
		c.cmdLogs(nick, hostmask, message)
	case cmd == "!logsearch":
		c.cmdLogSearch(nick, hostmask, message)
	case cmd == "!motd":
		c.cmdMotd(nick, hostmask, message)
	case cmd == "!version":
		c.cmdVersion(nick, hostmask, message)
	case cmd == "!login" || cmd == "!su":
		c.cmdLogin(nick, hostmask, message)
	case cmd == "!logout":
		c.cmdLogout(nick, hostmask, message)
	case cmd == "!set":
		c.cmdSet(nick, hostmask, message)
	case cmd == "!reload":
		c.cmdReload(nick, hostmask, message)
	case cmd == "!nick":
		c.cmdNick(nick, hostmask, message)
	case cmd == "!restart":
		c.cmdRestart(nick, hostmask, message)
	case cmd == "!shutdown":
		c.cmdShutdown(nick, hostmask, message)
	}
}

func (c *Client) cmdHelp(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	c.conn.Privmsg(nick, "Available commands:")
	c.conn.Privmsg(nick, "!summary - displays a summary of currently linked/missing servers")
	c.conn.Privmsg(nick, "!links - shows all currently connected servers, compared to the routing map")
	c.conn.Privmsg(nick, "!map - displays the most recent routing map")
	c.conn.Privmsg(nick, "!logs - displays the last 10 routing notices received")
	c.conn.Privmsg(nick, "!logs <number> - displays the last given number of messages")
	c.conn.Privmsg(nick, "!logsearch - search logs of routing notices for a given string")
	c.conn.Privmsg(nick, "!uplinks <server> - shows the primary, secondary and tertiary hubs for the specified server")
	c.conn.Privmsg(nick, "!motd - displays the MOTD from the routing team")
	c.conn.Privmsg(nick, "!version - displays bot version information")

	c.mu.RLock()
	isAdmin := c.admins[nick]
	c.mu.RUnlock()

	if isAdmin {
		c.conn.Privmsg(nick, " ")
		c.conn.Privmsg(nick, "Admin commands:")
		c.conn.Privmsg(nick, "!set motd <message>")
		c.conn.Privmsg(nick, "!reload - reload a fresh copy of the current routing map")
		c.conn.Privmsg(nick, "!nick - if you need to change my nick")
		c.conn.Privmsg(nick, "!restart")
		c.conn.Privmsg(nick, "!shutdown")
		c.conn.Privmsg(nick, "!logout")
	}
}

func (c *Client) cmdLinks(nick, hostmask, message string, summary bool) {
	c.logCommand(hostmask, message)

	// Reload map before checking
	c.reloadMap()

	// Initialize links collection
	c.linksMu.Lock()
	c.linksTree = routing.NewLinkTree()
	c.linksTarget = nick
	c.linksSummary = summary
	c.linksMu.Unlock()

	// Request LINKS from server
	c.conn.SendRaw("LINKS")
}

func (c *Client) cmdMap(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	c.mu.RLock()
	raw := c.routingMap.Raw
	c.mu.RUnlock()

	for _, line := range raw {
		c.conn.Privmsg(nick, line)
	}
}

func (c *Client) cmdUplinks(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	parts := strings.Fields(message)
	if len(parts) < 2 {
		// Show all servers with hubs
		c.mu.RLock()
		raw := c.routingMap.Raw
		c.mu.RUnlock()

		for _, line := range raw {
			// Skip header lines
			if strings.Contains(line, ":") &&
				!strings.HasPrefix(line, "Tier ") &&
				!strings.Contains(line, "Routing") &&
				!strings.HasPrefix(line, "Secondary ") {
				c.conn.Privmsg(nick, line)
			}
		}
		return
	}

	// Search for specific server
	server := parts[1]
	// Strip domain if present
	if idx := strings.Index(server, "."); idx > 0 {
		server = server[:idx]
	}
	// Clean up search term
	server = regexp.MustCompile(`[^\w\s-]`).ReplaceAllString(server, "")

	c.mu.RLock()
	raw := c.routingMap.Raw
	c.mu.RUnlock()

	found := 0
	serverLower := strings.ToLower(server)
	for _, line := range raw {
		lineLower := strings.ToLower(line)
		if strings.HasPrefix(lineLower, serverLower) && strings.Contains(line, ":") {
			if !strings.HasPrefix(line, "Tier ") &&
				!strings.Contains(line, "Routing") &&
				!strings.HasPrefix(line, "Secondary ") {
				c.conn.Privmsg(nick, line)
				found++
			}
		}
	}

	if found == 0 {
		c.conn.Privmsg(nick, "No such server found")
	}
}

func (c *Client) cmdLogs(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	parts := strings.Fields(message)
	count := 10
	if len(parts) > 1 {
		if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
			count = n
		}
	}

	c.mu.RLock()
	logs := c.logs
	c.mu.RUnlock()

	c.conn.Privmsg(nick, fmt.Sprintf("The last \x02%d\x02 routing notices:", count))

	for i := 0; i < count && i < len(logs); i++ {
		c.conn.Privmsg(nick, logs[i])
	}
}

func (c *Client) cmdLogSearch(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	parts := strings.SplitN(message, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		c.conn.Privmsg(nick, "Please specify a string to search for")
		return
	}

	term := strings.TrimSpace(parts[1])

	// Reject regex special characters
	if regexp.MustCompile(`[+|*()[\]]`).MatchString(term) {
		c.conn.Privmsg(nick, "Please try searching without regular expression characters - *+()|[]")
		return
	}

	c.mu.RLock()
	logs := c.logs
	c.mu.RUnlock()

	c.conn.Privmsg(nick, fmt.Sprintf("Displaying search results for \"%s\":", term))

	termLower := strings.ToLower(term)
	for _, log := range logs {
		if strings.Contains(strings.ToLower(log), termLower) {
			c.conn.Privmsg(nick, "    "+log)
		}
	}

	c.conn.Privmsg(nick, "End of matches")
}

func (c *Client) cmdMotd(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	c.mu.RLock()
	motd := c.motd
	c.mu.RUnlock()

	c.conn.Privmsg(nick, motd.Message)
	c.conn.Privmsg(nick, fmt.Sprintf("MOTD set by %s", motd.Setter))
}

func (c *Client) cmdVersion(nick, hostmask, message string) {
	c.logCommand(hostmask, message)

	c.conn.Privmsg(nick, fmt.Sprintf("rnexus version %s", Version))
	c.conn.Privmsg(nick, fmt.Sprintf("Built: %s", BuildDate))
	c.conn.Privmsg(nick, fmt.Sprintf("Commit: %s", GitCommit))
}

func (c *Client) cmdLogin(nick, hostmask, message string) {
	parts := strings.Fields(message)
	if len(parts) < 2 {
		c.conn.Privmsg(nick, "Usage: !login <password>")
		return
	}

	password := parts[1]

	if password == c.cfg.AdminPass {
		c.mu.Lock()
		c.admins[nick] = true
		c.mu.Unlock()

		c.conn.SendRaw(fmt.Sprintf("WATCH +%s", nick))
		c.conn.Privmsg(nick, "Password accepted, you are now an admin. Type !help for a list of admin-only commands")
		c.logCommand(hostmask, "successful login")
	} else {
		c.conn.Privmsg(nick, "Password incorrect")
		c.logCommand(hostmask, "INCORRECT LOGIN ATTEMPT")
	}
}

func (c *Client) cmdLogout(nick, hostmask, message string) {
	c.mu.Lock()
	isAdmin := c.admins[nick]
	delete(c.admins, nick)
	c.mu.Unlock()

	if isAdmin {
		c.conn.SendRaw(fmt.Sprintf("WATCH -%s", nick))
		c.conn.Privmsg(nick, "You have been logged out")
		c.logCommand(hostmask, "logged out")
	} else {
		c.conn.Privmsg(nick, "You're not logged in!")
		c.logCommand(hostmask, "tried to log out, but wasn't logged in")
	}
}

func (c *Client) cmdSet(nick, hostmask, message string) {
	c.mu.RLock()
	isAdmin := c.admins[nick]
	c.mu.RUnlock()

	// Check for !set motd
	messageLower := strings.ToLower(message)
	if strings.HasPrefix(messageLower, "!set motd") {
		if !isAdmin {
			c.conn.Privmsg(nick, "Sorry, only my admins can change the motd")
			c.logCommand(hostmask, "tried to change MOTD but wasn't logged in")
			return
		}

		// Extract the MOTD message
		parts := strings.SplitN(message, " ", 3)
		if len(parts) < 3 {
			c.conn.Privmsg(nick, "Usage: !set motd <message>")
			return
		}

		newMotd := strings.TrimSpace(parts[2])
		timestamp := time.Now().UTC().Format("Mon Jan 02, 2006 at 15:04:05 GMT")

		motd := &storage.MOTD{
			Setter:  fmt.Sprintf("%s on %s", nick, timestamp),
			Message: newMotd,
		}

		c.mu.Lock()
		c.motd = motd
		c.mu.Unlock()

		if err := storage.SaveMOTD(c.cfg.DataDir, motd); err != nil {
			c.conn.Privmsg(nick, fmt.Sprintf("Error saving MOTD: %v", err))
			return
		}

		c.conn.Privmsg(nick, fmt.Sprintf("MOTD has been set to \"%s\"", newMotd))
		c.logCommand(hostmask, fmt.Sprintf("changed MOTD to \"%s\"", newMotd))
	}
}

func (c *Client) cmdReload(nick, hostmask, message string) {
	c.mu.RLock()
	isAdmin := c.admins[nick]
	c.mu.RUnlock()

	if !isAdmin {
		c.conn.Privmsg(nick, "Sorry, only my admins can issue that command")
		c.logCommand(hostmask, "tried to reload routing map, but wasn't logged in")
		return
	}

	c.conn.Privmsg(nick, "Reloading routing map...")
	c.reloadMap()
	c.conn.Privmsg(nick, "Done.")
	c.logCommand(hostmask, "reloaded routing map")
}

func (c *Client) cmdNick(nick, hostmask, message string) {
	c.mu.RLock()
	isAdmin := c.admins[nick]
	c.mu.RUnlock()

	parts := strings.Fields(message)
	newNick := ""
	if len(parts) > 1 {
		newNick = parts[1]
	}

	if !isAdmin {
		c.conn.Privmsg(nick, "Sorry, only my admins can change my nick")
		c.logCommand(hostmask, fmt.Sprintf("nick change command to %s, not logged in", newNick))
		return
	}

	if newNick == "" {
		c.conn.Privmsg(nick, "Usage: !nick <newnick>")
		return
	}

	c.conn.SetNick(newNick)
	time.AfterFunc(time.Second, func() {
		c.conn.Privmsg(nick, fmt.Sprintf("Changed nick to %s", newNick))
	})
	c.logCommand(hostmask, fmt.Sprintf("nick change command to %s", newNick))
}

func (c *Client) cmdRestart(nick, hostmask, message string) {
	c.mu.RLock()
	isAdmin := c.admins[nick]
	c.mu.RUnlock()

	if !isAdmin {
		c.conn.Privmsg(nick, "Sorry, only my admins can restart me")
		c.logCommand(hostmask, "issued the restart command but wasn't logged in")
		return
	}

	c.logCommand(hostmask, "restart command")
	c.conn.Privmsg(nick, "Restarting")

	if c.OnRestart != nil {
		c.OnRestart()
	}
}

func (c *Client) cmdShutdown(nick, hostmask, message string) {
	c.mu.RLock()
	isAdmin := c.admins[nick]
	c.mu.RUnlock()

	if !isAdmin {
		c.conn.Privmsg(nick, "Sorry, only my admins can shut me down")
		c.logCommand(hostmask, "issued the shutdown command but wasn't logged in")
		return
	}

	c.logCommand(hostmask, message)
	c.conn.Privmsg(nick, "Shutting down")

	if c.OnShutdown != nil {
		c.OnShutdown()
	}
}

func (c *Client) reloadMap() {
	rmap, err := routing.LoadMap(c.cfg.DataDir)
	if err != nil {
		return
	}

	c.mu.Lock()
	c.routingMap = rmap
	c.mu.Unlock()
}
