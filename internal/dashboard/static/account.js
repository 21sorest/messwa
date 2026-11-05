'use strict';

// The account page: your mess API key, MCP tokens and password.

$('logo').append(svgEl(icon('paper')));
$('navIcon').append(svgEl(icon('prev')));
$('keyIcon').append(svgEl(icon('ingot')));
$('tokIcon').append(svgEl(icon('coin')));
$('pwIcon').append(svgEl(icon('check')));

const when = (iso) => {
  const t = new Date(iso);
  return !iso || t.getFullYear() < 2000 ? 'never' : t.toLocaleString('en-IN', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
};

function showErr(id, e) {
  const p = $(id);
  p.textContent = e ? e.message || String(e) : '';
  p.hidden = !e;
}

async function paintMe() {
  const m = await me();
  $('who').textContent = m.name;
  $('pwUser').value = m.name;
  $('role').textContent = m.role === 'admin'
    ? "Admin: you see everything, including the linked WhatsApp account's sales and DMs."
    : 'Member: you see the board and your own meals.';
  $('noSecret').hidden = m.can_store_keys;
  $('keyMeta').replaceChildren(m.mess_key_set ? el('b', '', 'saved') : el('span', 'red', 'not set'));
  $('key').placeholder = m.mess_key_set ? 'Paste a new key to replace it' : 'Paste your mess API key';
  $('keyClear').hidden = !m.mess_key_set;
}

async function refreshMe() {
  mePromise = null;
  await paintMe();
}

$('keyForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  showErr('keyErr');
  try {
    await send('PUT', 'api/mess-key', { key: $('key').value });
    $('key').value = '';
    await refreshMe();
  } catch (x) { showErr('keyErr', x); }
});

$('keyClear').addEventListener('click', async () => {
  if (!confirm('Remove your saved mess API key? Your meals page and the portal MCP tools stop working until you add one again.')) return;
  showErr('keyErr');
  try {
    await send('DELETE', 'api/mess-key');
    await refreshMe();
  } catch (x) { showErr('keyErr', x); }
});

async function loadTokens() {
  const r = await fetch('api/mcp-tokens', { cache: 'no-store' });
  if (r.status === 401) return toLogin();
  const ts = await r.json();
  const active = ts.filter((t) => !t.revoked);
  $('tokMeta').replaceChildren(el('b', '', String(active.length)), ` active`);
  $('tokens').hidden = !ts.length;
  const body = $('tokBody');
  body.replaceChildren();
  for (const t of ts) {
    const tr = el('tr', t.revoked ? 'revoked' : '');
    tr.append(el('td', '', t.label || '(no label)'), el('td', 'mono', `${t.prefix}…`), el('td', '', when(t.created_at)), el('td', '', when(t.last_used)));
    const td = el('td');
    if (t.revoked) td.append(el('span', 'dim', 'revoked'));
    else {
      const b = el('button', 'btn small', 'Revoke');
      b.type = 'button';
      b.addEventListener('click', async () => {
        if (!confirm(`Revoke "${t.label || t.prefix}"? Agents using it lose access right away.`)) return;
        try { await send('DELETE', `api/mcp-tokens/${t.id}`); loadTokens(); } catch (x) { showErr('tokErr', x); }
      });
      td.append(b);
    }
    tr.append(td);
    body.append(tr);
  }
}

$('tokForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  showErr('tokErr');
  try {
    const t = await send('POST', 'api/mcp-tokens', { label: $('label').value });
    const m = await me();
    $('label').value = '';
    $('freshTok').textContent = t.token;
    $('freshCmd').textContent = `claude mcp add --transport http messwa ${m.mcp_url} --header "Authorization: Bearer ${t.token}"`;
    $('fresh').hidden = false;
    loadTokens();
  } catch (x) { showErr('tokErr', x); }
});

$('pwForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  showErr('pwErr');
  $('pwOk').hidden = true;
  if ($('pwNew').value !== $('pwAgain').value) return showErr('pwErr', new Error("The new passwords don't match."));
  try {
    await send('POST', 'api/password', { current: $('pwOld').value, new: $('pwNew').value });
    for (const id of ['pwOld', 'pwNew', 'pwAgain']) $(id).value = '';
    $('pwOk').hidden = false;
  } catch (x) { showErr('pwErr', x); }
});

$('logout').addEventListener('click', async () => {
  try { await send('POST', 'api/logout'); } finally { location.href = 'login.html'; }
});

paintMe().then(loadTokens).catch(() => {});
