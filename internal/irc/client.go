package irc

import (
	"crypto/tls"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dalnet/rnexus/internal/config"
	"github.com/dalnet/rnexus/internal/routing"
	"github.com/dalnet/rnexus/internal/storage"
	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
)

// Version information (set at build time or here)
var (
	Version   = "1.0.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// Client represents the IRC bot client
type Client struct {
	conn   *ircevent.Connection
	cfg    *config.Config
	mu     sync.RWMutex
	ready  bool
	closed bool

	// Routing data
	routingMap *routing.Map
	logs       []string
	stats      []string
	motd       *storage.MOTD

	// Oper tracking: hostmask -> is oper
	opers map[string]bool
	// Admin session tracking: nick -> is admin
	admins map[string]bool

	// Pending WHOIS checks: nick -> {hostmask, message}
	pendingWhois map[string]*pendingCheck

	// Links collection state
	linksMu     sync.Mutex
	linksTree   *routing.LinkTree
	linksTarget string // Who requested !links
	linksSummary bool  // Whether this is a !summary request

	// Shutdown/restart callbacks
	OnShutdown func()
	OnRestart  func()
}

type pendingCheck struct {
	hostmask string
	message  string
}

// NewClient creates a new IRC client
func NewClient(cfg *config.Config) (*Client, error) {
	c := &Client{
		cfg:          cfg,
		opers:        make(map[string]bool),
		admins:       make(map[string]bool),
		pendingWhois: make(map[string]*pendingCheck),
	}

	// Load data files
	var err error
	c.routingMap, err = routing.LoadMap(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: could not load routing map: %v", err)
		c.routingMap = &routing.Map{Servers: make(map[string][]string)}
	}

	c.logs, err = storage.LoadLogs(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: could not load logs: %v", err)
	}

	c.stats, err = storage.LoadStats(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: could not load stats: %v", err)
	}

	c.motd, err = storage.LoadMOTD(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: could not load MOTD: %v", err)
		c.motd = &storage.MOTD{}
	}

	// Create IRC connection
	conn := &ircevent.Connection{
		Server:       fmt.Sprintf("%s:%d", cfg.Server, cfg.Port),
		Nick:         cfg.Nick,
		User:         cfg.Username,
		RealName:     cfg.IRCName,
		Password:     cfg.ServerPass,
		QuitMessage:  "Shutting down",
		Debug:        false,
		UseTLS:       false,
		TLSConfig:    &tls.Config{InsecureSkipVerify: true},
		SASLLogin:    "",
		SASLPassword: "",
	}
	c.conn = conn

	// Register handlers
	c.registerHandlers()

	return c, nil
}

func (c *Client) registerHandlers() {
	// Connected (end of MOTD)
	c.conn.AddCallback("376", c.onConnect)
	c.conn.AddCallback("422", c.onConnect) // MOTD missing is also "connected"

	// Private messages
	c.conn.AddCallback("PRIVMSG", c.onPrivMsg)

	// Notices (for routing notices)
	c.conn.AddCallback("NOTICE", c.onNotice)

	// WHOIS responses
	c.conn.AddCallback("313", c.onWhoisOper)    // RPL_WHOISOPERATOR
	c.conn.AddCallback("318", c.onWhoisEnd)     // RPL_ENDOFWHOIS

	// LINKS responses
	c.conn.AddCallback("364", c.onLinks)        // RPL_LINKS
	c.conn.AddCallback("365", c.onLinksEnd)     // RPL_ENDOFLINKS

	// Nick issues
	c.conn.AddCallback("432", c.onNickHeld)     // ERR_ERRONEUSNICKNAME
	c.conn.AddCallback("433", c.onNickInUse)    // ERR_NICKNAMEINUSE

	// WATCH logout notification
	c.conn.AddCallback("601", c.onWatchLogout)  // RPL_LOGOFF

	// CTCP VERSION
	c.conn.AddCallback("CTCP_VERSION", c.onCtcpVersion)
}

// Connect initiates the IRC connection
func (c *Client) Connect() error {
	return c.conn.Connect()
}

// Loop runs the IRC event loop (blocking)
func (c *Client) Loop() {
	c.conn.Loop()
}

// Quit disconnects from IRC
func (c *Client) Quit(message string) {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.conn.Quit()
}

func (c *Client) onConnect(e ircmsg.Message) {
	log.Println("Connected to IRC server")

	// Identify to NickServ
	if c.cfg.NickPass != "" {
		c.conn.Privmsg("NickServ", fmt.Sprintf("IDENTIFY %s %s", c.cfg.Nick, c.cfg.NickPass))
	}

	// OPER up
	if c.cfg.OperNick != "" && c.cfg.OperPass != "" {
		c.conn.SendRaw(fmt.Sprintf("OPER %s %s", c.cfg.OperNick, c.cfg.OperPass))
	}

	// Set user modes (+inFI, -hg)
	c.conn.Send("MODE", c.conn.CurrentNick(), "+inFI")
	c.conn.Send("MODE", c.conn.CurrentNick(), "-hg")

	c.mu.Lock()
	c.ready = true
	c.mu.Unlock()

	log.Println("Bot initialization complete")
}

func (c *Client) onPrivMsg(e ircmsg.Message) {
	if len(e.Params) < 2 {
		return
	}

	target := e.Params[0]
	message := e.Params[1]
	nick := e.Nick()
	nuh, err := e.NUH()
	if err != nil {
		return
	}
	hostmask := nuh.Canonical()

	// Only respond to private messages (not channel messages)
	if !strings.EqualFold(target, c.conn.CurrentNick()) {
		return
	}

	c.mu.RLock()
	isOper := c.opers[hostmask]
	c.mu.RUnlock()

	if isOper {
		// Known oper, process command directly
		c.handleCommand(nick, hostmask, message)
	} else {
		// Unknown user, initiate WHOIS check
		c.mu.Lock()
		c.pendingWhois[nick] = &pendingCheck{
			hostmask: hostmask,
			message:  message,
		}
		c.mu.Unlock()
		c.conn.Send("WHOIS", nick)
	}
}

func (c *Client) onWhoisOper(e ircmsg.Message) {
	// 313 <nick> :is an IRC operator
	if len(e.Params) < 2 {
		return
	}
	nick := e.Params[1]

	c.mu.Lock()
	pending := c.pendingWhois[nick]
	if pending != nil {
		c.opers[pending.hostmask] = true
	}
	c.mu.Unlock()

	// Process the pending command
	if pending != nil {
		c.handleCommand(nick, pending.hostmask, pending.message)
	}
}

func (c *Client) onWhoisEnd(e ircmsg.Message) {
	// 318 <nick> :End of /WHOIS list
	if len(e.Params) < 2 {
		return
	}
	nick := e.Params[1]

	c.mu.Lock()
	pending := c.pendingWhois[nick]
	delete(c.pendingWhois, nick)

	// Check if we got oper status
	var isOper bool
	if pending != nil {
		isOper = c.opers[pending.hostmask]
	}
	c.mu.Unlock()

	// If not an oper, log the attempt
	if pending != nil && !isOper {
		c.logCommand(pending.hostmask, fmt.Sprintf("USER - %s", pending.message))
	}
}

func (c *Client) onNotice(e ircmsg.Message) {
	if len(e.Params) < 2 {
		return
	}

	from := e.Source
	notice := e.Params[1]

	// Check for routing notices from DALnet servers
	if (strings.HasSuffix(from, "dal.net") || strings.HasSuffix(from, "upenn.edu")) &&
		strings.Contains(notice, "*** Routing") {

		// Parse the routing notice
		notice = strings.TrimPrefix(notice, "*** Routing -- from ")

		// Extract server name
		fromServer := from
		if idx := strings.Index(from, "."); idx > 0 {
			fromServer = from[:idx]
		}

		// Format timestamp
		timestamp := time.Now().UTC().Format("Mon Jan 02, 2006 15:04:05 GMT")
		logEntry := fmt.Sprintf("[%s] [%s]: %s", timestamp, fromServer, notice)

		c.mu.Lock()
		c.logs = storage.AddLog(c.logs, logEntry)
		c.mu.Unlock()

		// Save to file
		if err := storage.SaveLogs(c.cfg.DataDir, c.logs); err != nil {
			log.Printf("Error saving logs: %v", err)
		}
	}
}

func (c *Client) onLinks(e ircmsg.Message) {
	// 364 <me> <server> <hub> :<hops> <description>
	if len(e.Params) < 4 {
		return
	}

	server := e.Params[1]
	hub := e.Params[2]
	info := e.Params[3]

	// Parse hops and description
	var hops int
	var description string
	if parts := strings.SplitN(info, " ", 2); len(parts) >= 1 {
		hops, _ = strconv.Atoi(parts[0])
		if len(parts) > 1 {
			description = parts[1]
		}
	}

	c.linksMu.Lock()
	if c.linksTree != nil {
		c.linksTree.Add(server, hub, hops, description)
	}
	c.linksMu.Unlock()
}

func (c *Client) onLinksEnd(e ircmsg.Message) {
	// 365 <me> <mask> :End of /LINKS list
	c.linksMu.Lock()
	tree := c.linksTree
	target := c.linksTarget
	summary := c.linksSummary
	c.linksTree = nil
	c.linksTarget = ""
	c.linksSummary = false
	c.linksMu.Unlock()

	if tree == nil || target == "" {
		return
	}

	// Get the server we're connected to
	var connectedServer string
	if len(e.Params) > 0 {
		connectedServer = e.Source
	}

	c.mu.RLock()
	rmap := c.routingMap
	motd := c.motd
	c.mu.RUnlock()

	// Build and send tree (unless summary mode)
	if !summary {
		lines := tree.Build()
		for _, line := range lines {
			c.conn.Privmsg(target, line)
		}
		c.conn.Privmsg(target, "End of server list.")
		c.conn.Privmsg(target, fmt.Sprintf("Note - the map displayed above is the network as viewed from my server, %s", connectedServer))
	}

	// Compare against map
	total, linked, missing := routing.CompareToMap(tree, rmap)
	c.conn.Privmsg(target, fmt.Sprintf("Total servers: %d", total))
	c.conn.Privmsg(target, fmt.Sprintf("Linked servers: %d", linked))

	if len(missing) > 0 {
		c.conn.Privmsg(target, fmt.Sprintf("Missing servers: %s (%d)", strings.Join(missing, ", "), len(missing)))
	} else {
		c.conn.Privmsg(target, "No servers are currently missing")
	}

	// Show MOTD
	c.conn.Privmsg(target, " ")
	c.conn.Privmsg(target, fmt.Sprintf("[MOTD] %s", motd.Message))
	c.conn.Privmsg(target, fmt.Sprintf("MOTD set by %s", motd.Setter))
}

func (c *Client) onNickHeld(e ircmsg.Message) {
	if c.conn.CurrentNick() == c.cfg.Alternate {
		return
	}
	log.Printf("Nick is held, switching to alternate: %s", c.cfg.Alternate)
	c.conn.SetNick(c.cfg.Alternate)

	// Schedule nick recovery
	go func() {
		time.Sleep(15 * time.Second)
		c.conn.Privmsg("NickServ", fmt.Sprintf("RELEASE %s %s", c.cfg.Nick, c.cfg.NickPass))
		time.Sleep(2 * time.Second)
		c.conn.SetNick(c.cfg.Nick)
	}()
}

func (c *Client) onNickInUse(e ircmsg.Message) {
	if c.conn.CurrentNick() == c.cfg.Alternate {
		return
	}
	log.Printf("Nick in use, switching to alternate: %s", c.cfg.Alternate)
	c.conn.SetNick(c.cfg.Alternate)

	// Schedule nick recovery
	go func() {
		time.Sleep(15 * time.Second)
		c.conn.Privmsg("NickServ", fmt.Sprintf("GHOST %s %s", c.cfg.Nick, c.cfg.NickPass))
		time.Sleep(2 * time.Second)
		c.conn.SetNick(c.cfg.Nick)
	}()
}

func (c *Client) onWatchLogout(e ircmsg.Message) {
	// 601 <me> <nick> <user> <host> <timestamp> :logged out
	if len(e.Params) < 2 {
		return
	}
	nick := e.Params[1]

	// Check if this nick was a bot nick or admin
	c.mu.Lock()
	if strings.EqualFold(nick, c.cfg.Nick) {
		delete(c.admins, nick)
	}
	if c.admins[nick] {
		delete(c.admins, nick)
	}
	c.mu.Unlock()

	c.conn.SendRaw(fmt.Sprintf("WATCH -%s", nick))
}

func (c *Client) onCtcpVersion(e ircmsg.Message) {
	nick := e.Nick()
	reply := fmt.Sprintf("rnexus %s (built %s, commit %s)", Version, BuildDate, GitCommit)
	c.conn.SendRaw(fmt.Sprintf("NOTICE %s :\x01VERSION %s\x01", nick, reply))
}

func (c *Client) logCommand(hostmask, command string) {
	timestamp := time.Now().UTC().Format("Mon Jan 02, 2006 at 15:04:05 GMT")
	entry := fmt.Sprintf("%s: %s -> %s", timestamp, hostmask, command)

	c.mu.Lock()
	c.stats = storage.AddStat(c.stats, entry)
	c.mu.Unlock()

	if err := storage.SaveStats(c.cfg.DataDir, c.stats); err != nil {
		log.Printf("Error saving stats: %v", err)
	}
}
