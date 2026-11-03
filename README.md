# messwa

messwa links to WhatsApp as a companion device (via [whatsmeow](https://github.com/tulir/whatsmeow)), merges the mess buy/sell groups you pick into one deduplicated list of listings, and serves that list plus limited DM info to an agent over MCP.

The access limits live in the binary. Ingest drops every chat outside the allowed set before anything touches disk, and the MCP server reads only what ingest stored. So the agent can't reach other chats however it phrases a request.

The default `run` service and MCP tools are read-only: no messages, read receipts or presence are sent. Your phone keeps showing messages as unread and notifications don't change. The separate, explicitly armed `deliver-qr` command can send one image after a matching payment message.

## What gets stored

| Source | Stored |
| --- | --- |
| The configured mess groups | Posts that look like a listing (a sell/buy phrase or a meal name), reactions, edits, deletes |
| DMs with saved contacts | Name and last-message times only |
| DMs with unsaved numbers | Name, times, and message text (deleted after `dms.retention`, default 72h) |
| Every other group, status, channel, notes-to-self | Nothing |

If you save a number later, its stored DM text is deleted on its next message, and the MCP refuses to return it immediately.

## How listings work

**Dedupe.** Posts from one person for the same meal serving are one listing, however they're worded and across any number of groups. People post tomorrow's breakfast the evening before and again at 3am, so the lookback is 24 hours. Posts naming different messes, or veg and non-veg, stay separate. A sold or withdrawn listing never absorbs a new post, so a repost after a buyer backs out shows as available.

**Meal.** If the post names a meal ("lunch", "bf", "nashta"...), that meal's next serving wins. Otherwise it's the meal currently being served or the next one: a post at 10:00 is for lunch, one at 22:00 is for tomorrow's breakfast. `meal_source` says which rule applied.

**Status.** Status is computed when the agent asks, so un-reacting un-sells a listing.

- `sold` means the seller reacted to any copy with ✅, ✔, ☑ or 👍, or edited the post to say "sold", or replied "sold" to it. Ticks from anyone else don't count, but `other_ticks` reports them. Admins never tick other people's posts in these groups, so their ticks are ignored unless you set `admins_count = true`.
- `closed` means the seller reacted with ❌ to withdraw it.
- `expired` means the meal window has closed.
- `deleted` means every copy was deleted (shown only with `status: "all"`).
- `available` covers everything else.

Mess (Kadamba, Yuktahar, Palash, Bakul) and diet (veg, nonveg, egg) are tagged when the text mentions them, with typo tolerance. Palash and Yuktahar serve only veg, so their listings are always veg. Prices ("@30", "30/-", "free"), quantities ("x2") and exchanges ("have palash want kadamba") are parsed too, and a seller replying to their own post with a new price updates the listing.

## Dashboard

`run` also serves a read-only dashboard at `dashboard.listen` (set it to a private address so only your own network reaches it). It's laid out like an auction house's buy-it-now browser:

- The left column picks the mess, or all of them. Badges count what's available.
- Each slot is one listing, showing its price, "?" when none was stated, FREE, or SWAP with the mess wanted. The corners show the diet mark, the meal, and the quantity. The bar along the bottom drains as the meal's serving window runs out. Listings posted in the last 2 minutes shine, and an amber dot means someone other than the seller ticked it.
- The bottom row sorts by recency or price (click again to flip), filters by diet on Kadamba and Bakul, filters by meal, shows sold and withdrawn listings, and switches to buy requests.
- Hovering a slot shows the seller, phone, groups, times and the original text. Clicking opens every copy of the post and a button that opens WhatsApp with a draft to the seller. Nothing is sent from messwa.
- Below the board, the market pulse has a card per mess with messages per minute, sales per minute and demand (sales per message) over the last 15 minutes to 2 hours. A message is a post selling or swapping that mess. Each copy in each group counts, replies under a listing don't. A sale is the seller's tick or "sold". The strip shows messages (faint) and sales (solid) per 5 minutes over the last 2 hours, on one scale for every mess. The bottom row has the median asking price, the median time from post to sale, and buy orders per minute. HOT and SLOW compare a mess's demand with the whole market's. Clicking a card browses that mess.

Keys: `1`-`5` messes, arrows for pages, `R` `P` `D` `M` `S` `W` for the bottom row, `/` to search. Right click on the diet or meal button, or Shift with `D` or `M`, cycles backwards.

It shows sellers' names and phone numbers from the groups and nothing from DMs. It has no login, so don't put it on a public address.

To preview without the live service, `messwa dashboard -at "2026-11-02 12:45"` serves the board as it stood at that time on `127.0.0.1:8768`. Sold status reflects reactions up to now, not up to that time.

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

Needs Go 1.27+. On this server it's at `~/.local/go` (`export PATH=$HOME/.local/go/bin:$PATH`), the project lives in `~/messwa`, and Docker runs it (see below). The `messwa` wrapper in `~/.local/bin` runs the local binary from the project directory, which is fine for `dashboard -at` and `inspect` but not for `login`, `groups` or `run` while the container is up.

```sh
go build -o messwa .
cp config.example.toml config.toml

./messwa login                       # asks for your number (pairing code) or Enter for a QR
./messwa groups                      # copy the mess groups' JIDs into config.toml
./messwa run                         # serves http://127.0.0.1:8766/mcp
```

Stop `run` before using `groups`. Only one connection per linked device works at a time.

Add it to Claude Code:

```sh
claude mcp add --transport http messwa http://127.0.0.1:8766/mcp
```

### Docker

On this server the container is the live service. It uses host networking, so `config.toml` works unchanged: the dashboard binds the host's private address and MCP stays on loopback without a token. If that address isn't up yet at boot, the bind fails and Docker restarts the container until it is.

```sh
docker compose up -d --build        # rebuild and redeploy after code changes
docker compose logs -f
docker compose stop                 # before `login` or `groups`
docker compose run --rm messwa groups
```

To serve MCP beyond loopback, set `[mcp] listen = "0.0.0.0:8766"` and put `MESSWA_MCP_TOKEN=$(openssl rand -hex 32)` in `.env`. messwa refuses to start on a non-loopback address without it. Clients then pass `--header "Authorization: Bearer <token>"` to `claude mcp add`.

The old systemd unit (`~/.config/systemd/user/messwa.service`) is disabled. Don't run both: WhatsApp allows one connection per linked device.

`/healthz` returns 503 when the WhatsApp connection is down.

## Known limits

- History is mostly live-only. The first link gets a partial history sync (messages from the last 48h are ingested). After that, anything that arrives while messwa is down is lost.
- A sale agreed in DMs, where the seller never reacts or replies, still shows as available.
- Group members who appear only by LID (no phone number known) show a `jid` ending in `@lid` and no `phone`.
- whatsmeow is unofficial. A read-only device is low risk, but a ban is possible.

## Next: selling (phase 2)

### One-meal QR delivery

Stop `run` or `inspect-payment` before using the same linked session. Arm one
date/meal for a specific buyer and agreed INR amount with two decimal places:

```sh
./messwa deliver-qr -date 2026-11-04 -meal lunch \
  -peer '91XXXXXXXXXX@s.whatsapp.net' -amount 2.37
```

Without `-qr`, this sends an embedded image labelled **TEST IMAGE / NOT A MEAL
QR**. It works in the Docker image too. For a real sale, pass
`-qr state/meal-qr.png`; keep your real QR in the ignored state directory.
The image must be PNG or JPEG, no larger than 5 MiB or 8192 pixels per side.
The date is interpreted in the configured timezone and the configured meal's
end time is the delivery deadline. Nothing is posted to selling groups and
payment instructions are not sent by this command; agree the price first.

Only a new incoming WhatsApp Pay `sendPaymentMessage` from the selected buyer
with exactly that amount qualifies. Both the message and decoded transaction
timestamps must be after arming and before the meal ends. Normal text,
screenshots, other buyers, wrong amounts and old transactions do not qualify.
As agreed for this workflow, payment is **assumed**, with no bank receipt or
completion-status check. It sends the image back to that buyer's DM and exits.

The data database stores the buyer, amount, image hash, arming time, payment
message ID, outgoing image ID and state for each date/meal. It never stores
the raw payment payload. Repeated events and restarts cannot automatically
send the same slot twice. An armed slot can resume only with the same buyer,
price and image. A payment message cannot claim another meal.

Network failures or a crash after claiming leave the delivery as `sending`.
The command refuses to rearm `sending` or `sent` slots. Check the buyer's chat
and make any follow-up manually; a timeout does not prove that delivery failed.
The `sent` state records WhatsApp's send acknowledgement, not the buyer opening
the image. Keep the same state database when restarting. Do not run two
connections using the same linked-device session.

This command is not exposed through MCP and does not implement automatic
registration imports, scheduling, negotiation, buyer reassignment or refunds.

### Payment diagnostic

`inspect-payment` listens to one explicitly selected DM, including a saved
contact when selected. Stop `run` before reusing its session.

```sh
go build -o messwa .
# First setup only: copy config.example.toml to config.toml and run messwa login.
./messwa inspect-payment -peer '91XXXXXXXXXX@s.whatsapp.net'
```

Use the test sender's number with country code, or their full `@lid` JID.
Only new incoming messages from that peer are processed. Normal messages
produce field names only. A `sendPaymentMessage` can produce `decoded_prefix`
with currency, decimal amount, timestamp and an optional note. Amounts remain
strings to avoid floating-point rounding. The experimental decoder recognizes
the observed Android UPI Parcel layout, including `interopNote` when the
separate `noteMessage` is empty. Other layouts may not decode.

Header codes remain unmapped. `completion_status` is `unknown` and
`bank_verified` is always false. These are sender-provided message details,
not independent confirmation of a bank credit. No automatic sale or QR release
is implemented. The diagnostic does not attach the ingester, store DM content
in the data database, send chat messages, or expose extra MCP tools.

`-raw` includes raw payment fields in stdout for local debugging. It can expose
account details. Default output omits those fields but includes the decoded
amount and note. Keep output and session files private. Ctrl+C stops listening.

Validation used real incoming payments for INR 1.00, 3.00, 2.37 and 1.68, with
a note in the final sample. Fixtures contain only a non-identifying prefix and
synthetic note data. Tests cover malformed/truncated payloads, unknown note
wrappers, duplicate note keys and Unicode. No Java objects are deserialized.

The plan from `/home/sdixit/messbuysell/selling-instructions.md`, adapted to WhatsApp. You start a sell task (meal, price range). The agent posts "Selling Kadamba lunch" in the groups without a price, answers unsaved numbers who DM about it, negotiates within the range, then hands the chat to you. The binary enforces the rules: sends happen only during an active task, only to unsaved numbers who messaged first, never below the floor price, and rate-limited.
