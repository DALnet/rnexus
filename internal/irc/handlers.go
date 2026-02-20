package irc

// This file contains documentation for the IRC event handlers.
// The actual handler implementations are split across:
// - client.go: Connection lifecycle, WHOIS, LINKS, NOTICE handlers
// - commands.go: Bot command implementations

/*
Handler Summary:

Connection Events:
- 376/422 (onConnect): End of MOTD / MOTD missing - bot is connected
  - Identifies to NickServ
  - OPERs up
  - Sets user modes (+inFI, -hg)

Private Messages:
- PRIVMSG (onPrivMsg): Handles private messages from users
  - Checks if sender is known IRC operator (cached)
  - If not known, initiates WHOIS check
  - If known oper, routes to command handler

WHOIS Responses:
- 313 (onWhoisOper): RPL_WHOISOPERATOR - User is an IRC operator
  - Caches oper status by hostmask
  - Processes pending command
- 318 (onWhoisEnd): RPL_ENDOFWHOIS - End of WHOIS response
  - Cleans up pending check
  - Logs non-oper access attempts

Server Notices:
- NOTICE (onNotice): Handles server notices
  - Filters for routing notices from DALnet servers
  - Parses and logs routing information

LINKS Responses:
- 364 (onLinks): RPL_LINKS - Server link information
  - Collects server topology data
- 365 (onLinksEnd): RPL_ENDOFLINKS - End of LINKS response
  - Builds and displays server tree
  - Compares against routing map
  - Shows missing servers

Nick Issues:
- 432 (onNickHeld): ERR_ERRONEUSNICKNAME - Nick is held
  - Switches to alternate nick
  - Schedules RELEASE and nick change
- 433 (onNickInUse): ERR_NICKNAMEINUSE - Nick in use
  - Switches to alternate nick
  - Schedules GHOST and nick change

Admin Session:
- 601 (onWatchLogout): RPL_LOGOFF - WATCH notification
  - Auto-logs out admin if they quit/change nick

CTCP:
- CTCP_VERSION: Responds with bot version information
*/
