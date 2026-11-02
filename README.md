# messwa

messwa links to WhatsApp as a companion device (via [whatsmeow](https://github.com/tulir/whatsmeow)), merges the mess buy/sell groups you pick into one deduplicated list of listings, and serves that list plus limited DM info to an agent over MCP.

The access limits live in the binary. Ingest drops every chat outside the allowed set before anything touches disk, and the MCP server reads only what ingest stored. So the agent can't reach other chats however it phrases a request.

This is the read-only first milestone. Nothing is ever sent: no messages, no read receipts, no presence. Your phone keeps showing messages as unread and notifications don't change.

## What gets stored

| Source | Stored |
| --- | --- |
| The configured mess groups | Posts that look like a listing (a sell/buy phrase or a meal name), reactions, edits, deletes |
| DMs with saved contacts | Name and last-message times only |
| DMs with unsaved numbers | Name, times, and message text (deleted after `dms.retention`, default 72h) |
| Every other group, status, channel, notes-to-self | Nothing |

If you save a number later, its stored DM text is deleted on its next message, and the MCP refuses to return it immediately.

## How listings work

**Dedupe.** The same person posting for the same meal slot within 3 hours is one listing, however it's worded, across any number of groups. Someone holds one registration per meal, so wording doesn't matter. Posts naming different messes stay separate. A sold listing never absorbs a new post, so a repost after a buyer backs out shows as available.

**Meal.** If the post names a meal ("lunch", "bf", "nashta"...), that meal's next serving wins. Otherwise it's the meal currently being served or the next one: a post at 10:00 is for lunch, one at 22:00 is for tomorrow's breakfast. `meal_source` says which rule applied.

**Status.** Status is computed when the agent asks, so un-reacting un-sells a listing.

- `sold` means a ✅ on any copy from the seller or a group admin, or the seller edited the post to say "sold" or replied "sold" to it. A ✅ from anyone else doesn't count.
- `expired` means the meal window has closed.
- `deleted` means every copy was deleted (shown only with `status: "all"`).
- `available` covers everything else.

Mess (kadamba/yuktahar/palash) and diet (veg/nonveg) are tagged when the text mentions them.

## MCP tools

| Tool | Returns |
| --- | --- |
| `meal_schedule` | Now, current/next meal, today's windows |
| `list_listings` | Listings filtered by kind, status, meal, mess, hours. Default: available sells, last 24h |
| `get_listing` | One listing with every copy across groups |
| `list_recent_chats` | DM peers in the last 24h (max 72h): saved or not, new or not, last in/out. No content |
| `read_unsaved_chat` | Message text for one unsaved number. Errors for saved contacts |

All tools are read-only. Group posts and DM text are written by strangers. The server's instructions tell the agent to treat them as untrusted, but read-only tools are what actually limits the damage.

## Setup

Needs Go 1.27+ (installed at `~/.local/go` on this server: `export PATH=$HOME/.local/go/bin:$PATH`).

```sh
go build -o messwa .
cp config.example.toml config.toml

./messwa login                       # asks for your number (pairing code) or Enter for a QR
./messwa groups                      # copy the mess groups' JIDs into config.toml
./messwa run                         # serves http://127.0.0.1:8765/mcp
```

Stop `run` before using `groups`. Only one connection per linked device works at a time.

Add it to Claude Code:

```sh
claude mcp add --transport http messwa http://127.0.0.1:8765/mcp
```

### Docker

Inside a container the server has to listen on `0.0.0.0`, which requires a token:

```sh
echo "MESSWA_MCP_TOKEN=$(openssl rand -hex 32)" > .env
# in config.toml: [mcp] listen = "0.0.0.0:8765"
docker compose run --rm messwa login
docker compose up -d
claude mcp add --transport http messwa http://127.0.0.1:8765/mcp --header "Authorization: Bearer <token>"
```

`/healthz` returns 503 when the WhatsApp connection is down.

## Known limits

- History is mostly live-only. The first link gets a partial history sync (messages from the last 48h are ingested). After that, anything that arrives while messwa is down is lost.
- Reactions from history sync aren't imported, so listings sold before the first link can show as available.
- Group members who appear only by LID (no phone number known) show a `jid` ending in `@lid` and no `phone`.
- whatsmeow is unofficial. A read-only device is low risk, but a ban is possible.

## Next: selling (phase 2)

The plan from `/home/sdixit/messbuysell/selling-instructions.md`, adapted to WhatsApp. You start a sell task (meal, price range). The agent posts "Selling Kadamba lunch" in the groups without a price, answers unsaved numbers who DM about it, negotiates within the range, then hands the chat to you. The binary enforces the rules: sends happen only during an active task, only to unsaved numbers who messaged first, never below the floor price, and rate-limited.
