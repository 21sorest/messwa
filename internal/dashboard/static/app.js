'use strict';

// Mess Auction House. Everything people wrote in the groups is untrusted, so
// it only ever reaches the page through textContent.

const $ = (id) => document.getElementById(id);
function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
}
function svgEl(markup) {
  const t = document.createElement('template');
  t.innerHTML = markup.trim(); // only our own icon markup, never user text
  return t.content.firstChild;
}
const cap = (s) => (s ? s[0].toUpperCase() + s.slice(1) : '');

// ---------- pixel art ----------

function pix(rows, pal) {
  const H = rows.length, W = rows[0].length;
  let r = '';
  rows.forEach((row, y) => {
    for (let x = 0; x < W;) {
      const c = row[x];
      if (c === '.') { x++; continue; }
      let n = 1;
      while (x + n < W && row[x + n] === c) n++;
      r += `<rect x="${x}" y="${y}" width="${n}" height="1" fill="${pal[c]}"/>`;
      x += n;
    }
  });
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" shape-rendering="crispEdges" aria-hidden="true">${r}</svg>`;
}

const ART = {
  kadamba: [[
    '...o..o.o...',
    '.o.oooooo.o.',
    '..owwoooooo.',
    '.owwoooooooo',
    'oowoooooooO.',
    '.oooooooOOOo',
    'oooooooOOOO.',
    '.ooooOOOOOd.',
    '..oOOOOOdd..',
    '.o..dddd..o.',
    '.....gG.....',
    '....gGG.....',
  ], { o: '#f7c548', O: '#d99a1e', w: '#fff0b3', d: '#a8721a', g: '#5aa85d', G: '#2f6b34' }],
  palash: [[
    '.......r....',
    '......rr....',
    '.....rrr..r.',
    '....rryr.rr.',
    '...rryyrrrr.',
    '..rryyyyrr..',
    '..ryylyyrR..',
    '..ryyyyrrR..',
    '..rryyrrRR..',
    '...rrrrRR...',
    '....RRkk....',
    '.....kk.....',
  ], { r: '#ff5a36', R: '#c63a1f', y: '#ffb04a', l: '#ffe2a8', k: '#6b3a2a' }],
  yuktahar: [[
    '...SSSSSS...',
    '..SssssssS..',
    '.SsyysgggsS.',
    'SsyyysggggsS',
    'SsyyysggggsS',
    'SssysssggssS',
    'SsbbswwwsrsS',
    'SbbbwwwwsrrS',
    'SsbbwwwwsrsS',
    '.SsswwwsssS.',
    '..SssssssS..',
    '...SSSSSS...',
  ], { S: '#7f8b9b', s: '#cdd5df', y: '#f2c14e', g: '#5bb55b', w: '#f6f3ea', b: '#c88d4e', r: '#e2553f' }],
  bakul: [[
    '.....ll.....',
    '.....lL.....',
    '..l..lL..l..',
    '...lLLLLl...',
    '....LccL....',
    'llLLLccLLLll',
    '..DLLccLLD..',
    '...lLLLLl...',
    '..l..LD..l..',
    '.....LD.....',
    '.....D......',
    '............',
  ], { l: '#efe6ff', L: '#b89cff', D: '#8466e0', c: '#f7c548' }],
  all: [[
    '............',
    '.kkkk..yyyy.',
    '.kkkk..yyyy.',
    '.kkkk..yyyy.',
    '.kkkk..yyyy.',
    '............',
    '............',
    '.pppp..bbbb.',
    '.pppp..bbbb.',
    '.pppp..bbbb.',
    '.pppp..bbbb.',
    '............',
  ], { k: '#f7c548', y: '#36c9b0', p: '#ff5a36', b: '#b89cff' }],

  breakfast: [[
    'y..y..y',
    '.y.y.y.',
    '..yyy..',
    '.yyyyy.',
    'yyyyyyy',
    'hhhhhhh',
    '.......',
  ], { y: '#ffc94a', h: '#7f93b3' }],
  lunch: [[
    'y..y..y',
    '.yyyyy.',
    '.yyyyy.',
    'yyyyyyy',
    '.yyyyy.',
    '.yyyyy.',
    'y..y..y',
  ], { y: '#ffd84a' }],
  snacks: [[
    '...a...',
    '..aaa..',
    '..aaa..',
    '.aaaAa.',
    '.aaAaa.',
    'aaaaaAa',
    'AAAAAAA',
  ], { a: '#e8a447', A: '#a8681f' }],
  dinner: [[
    '..mmmm.',
    '.mmm...',
    'mmm....',
    'mmm....',
    'mmm....',
    '.mmm...',
    '..mmmm.',
  ], { m: '#dfe4f2' }],

  clock: [[
    '...RRRRRR...',
    '..RffffffR..',
    '.RffffhfffR.',
    'RfffffhffffR',
    'RfffffhffffR',
    'RfffffhhhffR',
    'RffffffffffR',
    'RffffffffffR',
    '.RffffffffR.',
    '..RffffffR..',
    '...RRRRRR...',
    '............',
  ], { R: '#e8c25a', f: '#f3efe2', h: '#2b2f3a' }],
  coin: [[
    '....GGGG....',
    '..GGggggGG..',
    '.GgwwgggggG.',
    '.GwgddddggG.',
    'GgwdggggdggG',
    'GggdggggdggG',
    'GggdggggdggG',
    'GggdggggdggG',
    '.GggddddggG.',
    '.GgggggggGG.',
    '..GGggggGG..',
    '....GGGG....',
  ], { G: '#c98a12', g: '#ffc53d', w: '#fff2b8', d: '#d99a1e' }],
  ingot: [[
    '............',
    '............',
    '...GGGGGGGG.',
    '..GwwwggggGG',
    '.GwggggggGdG',
    'GGGGGGGGGddG',
    'GwwwgggggGdG',
    'GwgggggggGd.',
    'GggggggggG..',
    'GGGGGGGGGG..',
    '............',
    '............',
  ], { G: '#c98a12', g: '#ffc53d', w: '#fff2b8', d: '#9a6a0e' }],
  paper: [[
    '............',
    '..PPPPPPPP..',
    '..PppppppP..',
    '..PpkkkkpP..',
    '..PppppppP..',
    '..PpkkkkpP..',
    '..PppppppP..',
    '..PpkkkppP..',
    '..PppppppP..',
    '..PPPPPPPP..',
    '............',
    '............',
  ], { P: '#b9a77a', p: '#efe3c2', k: '#6b5a33' }],
  check: [[
    '............',
    '..........cc',
    '.........cc.',
    '........cc..',
    '.......cc...',
    'cc....cc....',
    '.cc..cc.....',
    '..cccc......',
    '...cc.......',
    '............',
    '............',
    '............',
  ], { c: '#5be37a' }],
  next: [[
    '............',
    '.....a......',
    '.....aa.....',
    '.....aaa....',
    'aaaaaaaaa...',
    'aaaaaaaaaa..',
    'aaaaaaaaa...',
    '.....aaa....',
    '.....aa.....',
    '.....a......',
    '............',
    '............',
  ], { a: '#dfe4f2' }],
};
ART.prev = [ART.next[0].map((r) => [...r].reverse().join('')), ART.next[1]];
const icon = (name) => pix(ART[name][0], ART[name][1]);

function mealsAllIcon() {
  const sub = ['breakfast', 'lunch', 'snacks', 'dinner'].map((m, i) => {
    const s = icon(m).replace('<svg ', `<svg x="${(i % 2) * 8}" y="${Math.floor(i / 2) * 8}" width="7" height="7" `);
    return s;
  }).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 15 15" aria-hidden="true">${sub}</svg>`;
}

// Indian food marks: green dot for veg, brown triangle for non-veg.
function dietMark(d) {
  const col = { veg: '#34c463', nonveg: '#c9513a', egg: '#f2b51b' }[d];
  if (!col) return '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" aria-hidden="true"><rect x="1" y="1" width="10" height="10" rx="1.5" fill="none" stroke="#4a5266" stroke-width="1.3" stroke-dasharray="2 1.6"/></svg>';
  const inner = d === 'nonveg'
    ? `<polygon points="6,3 9.2,8.6 2.8,8.6" fill="${col}"/>`
    : `<circle cx="6" cy="6" r="2.7" fill="${col}"/>`;
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 12 12" aria-hidden="true"><rect x="1" y="1" width="10" height="10" rx="1.5" fill="none" stroke="${col}" stroke-width="1.4"/>${inner}</svg>`;
}

const waIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M12 2a10 10 0 0 0-8.6 15.1L2 22l5-1.3A10 10 0 1 0 12 2Zm0 18.2a8.2 8.2 0 0 1-4.2-1.2l-.3-.2-3 .8.8-2.9-.2-.3A8.2 8.2 0 1 1 12 20.2Zm4.5-6.1c-.2-.1-1.5-.7-1.7-.8-.2-.1-.4-.1-.6.1l-.8 1c-.1.2-.3.2-.5.1a6.7 6.7 0 0 1-3.3-2.9c-.2-.4.2-.4.7-1.3.1-.2 0-.3 0-.4l-.8-1.8c-.2-.5-.4-.4-.6-.4h-.5a1 1 0 0 0-.7.3 3 3 0 0 0-.9 2.2 5.2 5.2 0 0 0 1.1 2.7 11.8 11.8 0 0 0 4.5 4c1.7.7 2.3.8 3.2.6a2.7 2.7 0 0 0 1.8-1.3 2.2 2.2 0 0 0 .2-1.3c-.1-.1-.3-.2-.5-.3Z"/></svg>';

// ---------- mess identities ----------

const MESS = {
  kadamba: { color: '#f7c548', blurb: 'Golden kadamba blossoms' },
  yuktahar: { color: '#36c9b0', blurb: 'The balanced thali' },
  palash: { color: '#ff6a45', blurb: 'Flame of the forest' },
  bakul: { color: '#b89cff', blurb: 'Star-shaped bakul flowers' },
  all: { color: '#e8ecf3', blurb: 'Every mess at once' },
};
const messColor = (m) => (MESS[m] || {}).color || '#9aa3b5';
const messIcon = (m) => (ART[m] ? icon(m) : pix(['xxxx', 'xxxx', 'xxxx', 'xxxx'], { x: messColor(m) }));
const MEALS = ['breakfast', 'lunch', 'snacks', 'dinner'];
const DIETS = ['all', 'veg', 'nonveg', 'egg'];
const DIET_LABEL = { all: 'All', veg: 'Veg', nonveg: 'Non-veg', egg: 'Egg' };
const DIET_COLOR = { veg: 'ca', nonveg: 'cc', egg: 'c6' };

// ---------- state ----------

const saved = (() => { try { return JSON.parse(localStorage.getItem('mah') || '{}'); } catch { return {}; } })();
const S = Object.assign({ tab: 'kadamba', mode: 'sell', sort: 'recent', recentDir: 'desc', priceDir: 'asc', diet: 'all', meal: 'all', sold: false }, saved, { page: 0, q: '' });
const persist = () => {
  const { tab, mode, sort, recentDir, priceDir, diet, meal, sold } = S;
  localStorage.setItem('mah', JSON.stringify({ tab, mode, sort, recentDir, priceDir, diet, meal, sold }));
};

let D = null;          // last /api/state
let skew = 0;          // server clock minus ours, seconds
let lastOK = 0;
let failed = false;
const seen = new Set();
let popIds = new Set();
let firstLoad = true;
let L = { s: 72, gap: 6, cols: 9, rows: 5 };
let byId = new Map();
let pageItems = [];
let pages = 1;

const now = () => Date.now() / 1000 + skew;
const tz = () => (D && D.tz) || 'Asia/Kolkata';
const hm = (u) => new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false, timeZone: tz() }).format(u * 1000);
const ymd = (u) => new Intl.DateTimeFormat('en-CA', { timeZone: tz() }).format(u * 1000);
function dayWord(date) {
  const n = now();
  if (date === ymd(n)) return 'today';
  if (date === ymd(n + 86400)) return 'tomorrow';
  if (date === ymd(n - 86400)) return 'yesterday';
  return date;
}
function dur(sec) {
  const m = Math.max(1, Math.round(sec / 60));
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, '0')}m`;
}
function ago(u) {
  const s = now() - u;
  return s < 60 ? 'just now' : `${dur(s)} ago`;
}
const vegOnly = (m) => !!(D && D.messes.find((x) => x.name === m && x.veg_only));
const window_ = (meal) => (D ? D.today.find((s) => s.meal === meal) : null);
const shortGroup = (g) => g.replace(/^mess\s+/i, '').replace(/\s*@\s*/, ' @ ');

// ---------- data ----------

// The server pushes a "change" event on api/events whenever a post, edit,
// delete or reaction lands, and the page fetches the board then. While that
// stream is up, a slow refresh only catches meals ending. If it drops, poll
// every 5s while a meal is being served and every 30s otherwise, cutting the
// slow wait short so polling speeds up as soon as the next meal starts.
const FAST = 5, SLOW = 30, IDLE = 60;
let pollTimer = 0;
let streaming = false;
let loading = false;
let again = false;
function pollEvery() {
  if (streaming) return IDLE;
  if (!D) return FAST;
  const n = now();
  if (D.today.some((s) => s.start <= n && n < s.end)) return FAST;
  const next = D.today.filter((s) => s.start > n).map((s) => s.start - n);
  return Math.max(FAST, Math.min(SLOW, ...next));
}
function schedule() {
  clearTimeout(pollTimer);
  pollTimer = setTimeout(load, pollEvery() * 1000);
}

async function load() {
  if (loading) { again = true; return; }
  loading = true;
  schedule();
  try {
    const r = await fetch('api/state', { cache: 'no-store' });
    if (!r.ok) throw new Error(r.status);
    const d = await r.json();
    skew = d.now - Date.now() / 1000;
    popIds = new Set();
    for (const l of d.listings) {
      if (!firstLoad && !seen.has(l.id)) popIds.add(l.id);
      seen.add(l.id);
    }
    D = d;
    byId = new Map(d.listings.map((l) => [l.id, l]));
    firstLoad = false;
    failed = false;
    lastOK = Date.now();
    if (!D.messes.some((m) => m.name === S.tab) && S.tab !== 'all') S.tab = D.messes[0] ? D.messes[0].name : 'all';
    render();
  } catch (e) {
    failed = true;
    tickLive();
  } finally {
    loading = false;
    if (again) { again = false; load(); }
  }
}

function connect() {
  if (typeof EventSource === 'undefined') return;
  const es = new EventSource('api/events');
  es.addEventListener('open', () => {
    streaming = true;
    if (lastOK) load(); // catch up on anything missed while disconnected
    tickLive();
  });
  es.addEventListener('change', () => {
    if (document.visibilityState === 'visible') load();
  });
  es.addEventListener('error', () => {
    // The browser reconnects on its own; poll until it does.
    streaming = false;
    schedule();
    tickLive();
  });
}

function inMode(l) {
  return S.mode === 'sell' ? l.kind !== 'buy' : l.kind === 'buy';
}

// matches reports whether l shows on the given tab under the current filters.
function matches(l, tab) {
  if (!inMode(l)) return false;
  if (tab !== 'all' && l.mess !== tab) return false;
  if (!S.sold && l.status !== 'available') return false;
  if (S.meal !== 'all' && l.meal !== S.meal) return false;
  if (S.diet !== 'all' && (tab === 'all' || !vegOnly(tab)) && (l.diet || '') !== S.diet) return false;
  if (S.q) {
    const hay = [l.text, l.seller.name, l.seller.phone, l.mess, l.meal, ...(l.groups || [])].join(' ').toLowerCase();
    if (!S.q.toLowerCase().split(/\s+/).every((w) => hay.includes(w))) return false;
  }
  return true;
}

function sorted(list) {
  const rank = (l) => (l.status === 'available' ? 0 : 1);
  const recent = (a, b) => b.last_seen_at - a.last_seen_at;
  const cmp = S.sort === 'price'
    ? (a, b) => {
      const pa = a.price == null ? null : a.price, pb = b.price == null ? null : b.price;
      if (pa == null && pb == null) return recent(a, b);
      if (pa == null) return 1;
      if (pb == null) return -1;
      return (S.priceDir === 'asc' ? pa - pb : pb - pa) || recent(a, b);
    }
    : (a, b) => (S.recentDir === 'desc' ? recent(a, b) : -recent(a, b));
  return list.slice().sort((a, b) => rank(a) - rank(b) || cmp(a, b));
}

// ---------- layout ----------

function layout() {
  const w = window.innerWidth;
  const s = w < 420 ? 50 : w < 640 ? 56 : w < 1000 ? 64 : 72;
  const gap = w < 640 ? 5 : 6;
  const pad = w < 640 ? 20 + 22 : 40 + 34;
  const avail = Math.min(w, 1280) - pad - s - gap * 2;
  const cols = Math.max(4, Math.min(12, Math.floor((avail + gap) / (s + gap))));
  const rows = cols <= 6 ? 7 : 5;
  L = { s, gap, cols, rows };
  const st = document.documentElement.style;
  st.setProperty('--s', s + 'px');
  st.setProperty('--gap', gap + 'px');
  st.setProperty('--cols', cols);
  st.setProperty('--rows', rows);
}

// ---------- render ----------

function render() {
  if (!D) return;
  const accent = messColor(S.tab);
  document.documentElement.style.setProperty('--accent', accent);
  renderDay();
  renderTabs();
  renderGrid();
  renderControls();
  tickLive();
}

function renderDay() {
  const n = now();
  const parts = new Intl.DateTimeFormat('en-GB', { hour: 'numeric', minute: 'numeric', hour12: false, timeZone: tz() }).formatToParts(n * 1000);
  const get = (t) => +parts.find((p) => p.type === t).value;
  const midnight = n - (get('hour') * 3600 + get('minute') * 60) - (n % 60);
  const t0 = midnight + 6 * 3600, t1 = midnight + 24 * 3600;
  const pct = (u) => `${Math.max(0, Math.min(100, ((u - t0) / (t1 - t0)) * 100))}%`;

  const track = $('track');
  track.replaceChildren();
  for (const s of D.today) {
    const seg = el('div', 'seg');
    if (n > s.end) seg.classList.add('past');
    if (n >= s.start && n <= s.end) seg.classList.add('now');
    seg.style.setProperty('left', pct(s.start));
    seg.style.setProperty('width', `calc(${pct(s.end)} - ${pct(s.start)})`);
    track.append(seg);
    const g = el('span', 'seg-glyph' + (seg.classList.contains('now') ? ' now' : ''));
    g.style.setProperty('left', `calc((${pct(s.start)} + ${pct(s.end)}) / 2)`);
    g.append(svgEl(icon(s.meal)));
    g.title = `${cap(s.meal)} ${hm(s.start)}-${hm(s.end)}`;
    track.append(g);
  }
  const mark = el('div', 'now-mark');
  mark.style.setProperty('left', pct(n));
  track.append(mark);

  const lab = $('dayLabel');
  lab.replaceChildren();
  if (D.current) {
    lab.append(el('b', '', cap(D.current.meal)), ' is being served. ',
      el('span', 'gold', `${dur(D.current.end - n)} left`), `, until ${hm(D.current.end)}`);
  } else if (D.next) {
    lab.append('Next up: ', el('b', '', cap(D.next.meal)), ` at ${hm(D.next.start)} ${dayWord(D.next.date) === 'today' ? '' : dayWord(D.next.date)}`,
      el('span', 'gold', ` (in ${dur(D.next.start - n)})`));
  }
}

function renderTabs() {
  const tabs = $('tabs');
  tabs.replaceChildren();
  const names = [...D.messes.map((m) => m.name), 'all'];
  names.forEach((name, i) => {
    const b = el('button', 'slot tab' + (S.tab === name ? ' on' : ''));
    b.style.setProperty('--m', messColor(name));
    b.append(svgEl(messIcon(name)));
    const count = D.listings.filter((l) => matches(l, name) && l.status === 'available').length;
    b.append(el('span', 'badge' + (count ? '' : ' zero'), String(count)));
    b.setAttribute('aria-label', `${cap(name)}: ${count} listings`);
    b.addEventListener('click', () => setTab(name));
    tip(b, () => tabTip(name, count, i));
    tabs.append(b);
  });
  for (let i = names.length; i < L.rows; i++) tabs.append(el('div', 'slot'));
}

function tabTip(name, count, i) {
  const what = S.mode === 'sell' ? 'for sale' : 'wanted';
  const lines = [
    [{ t: name === 'all' ? 'All messes' : cap(name), c: 'nm', color: messColor(name) }],
    [{ t: (MESS[name] || {}).blurb || '', c: 'c8' }],
    0,
    [{ t: 'Listings: ', c: 'c7' }, { t: `${count} ${what}`, c: 'cf' }],
  ];
  if (name !== 'all') lines.push([{ t: 'Serves: ', c: 'c7' }, vegOnly(name) ? { t: 'veg only', c: 'ca' } : { t: 'veg and non-veg', c: 'cf' }]);
  lines.push(0, [{ t: S.tab === name ? 'Currently browsing' : 'Click to browse!', c: 'ce' }]);
  lines.push({ rar: `KEY ${i + 1}`, c: 'c8' });
  return lines;
}

function renderGrid() {
  const grid = $('grid');
  const list = sorted(D.listings.filter((l) => matches(l, S.tab)));
  const per = L.cols * L.rows;
  pages = Math.max(1, Math.ceil(list.length / per));
  S.page = Math.min(S.page, pages - 1);
  pageItems = list.slice(S.page * per, (S.page + 1) * per);

  const nodes = pageItems.map(slotNode);
  while (nodes.length < per) nodes.push(el('div', 'slot'));
  grid.replaceChildren(...nodes);

  if (!list.length) {
    const box = el('div', 'empty');
    const inner = el('div');
    inner.append(el('b', '', S.q ? 'NO MATCHES' : 'NOTHING HERE'), emptyText());
    box.append(inner);
    grid.append(box);
  }

  const avail = list.filter((l) => l.status === 'available').length;
  const tabName = S.tab === 'all' ? 'All messes' : cap(S.tab);
  $('headIcon').replaceChildren(svgEl(messIcon(S.tab)));
  $('headName').textContent = tabName;
  const bits = [`${avail} ${S.mode === 'sell' ? 'for sale' : 'wanted'}`];
  if (S.meal !== 'all') bits.push(S.meal);
  if (S.diet !== 'all' && (S.tab === 'all' || !vegOnly(S.tab))) bits.push(DIET_LABEL[S.diet].toLowerCase());
  if (S.q) bits.push(`matching "${S.q}"`);
  $('headMeta').textContent = bits.join(' · ');
  $('pgText').textContent = `${S.page + 1}/${pages}`;
  $('pgPrev').disabled = S.page === 0;
  $('pgNext').disabled = S.page >= pages - 1;
  $('sub').textContent = S.mode === 'sell' ? 'Buy it now' : 'Buy orders';
}

function emptyText() {
  const where = S.tab === 'all' ? 'any mess' : cap(S.tab);
  const meal = S.meal === 'all' ? '' : ` ${S.meal}`;
  return S.mode === 'sell' ? `No${meal} listings for ${where} right now.` : `Nobody is asking for${meal} ${where} right now.`;
}

function slotNode(l) {
  const b = el('button', 'slot item');
  b.dataset.id = l.id;
  b.style.setProperty('--m', messColor(l.mess));
  if (l.status === 'sold' || l.status === 'closed') b.classList.add(l.status);
  if (now() - l.first_seen_at < 15 * 60 && l.status === 'available') b.classList.add('fresh');
  if (popIds.has(l.id)) b.classList.add('pop');

  if (!vegOnly(l.mess) || S.tab === 'all') {
    const d = el('span', 'diet');
    d.append(svgEl(dietMark(l.diet)));
    b.append(d);
  }
  if (ART[l.meal]) {
    const m = el('span', 'mealg');
    m.append(svgEl(icon(l.meal)));
    b.append(m);
  }
  b.append(priceNode(l));
  if (S.tab === 'all') {
    const m = el('span', 'messg');
    m.append(svgEl(messIcon(l.mess)));
    b.append(m);
  }
  if (l.quantity > 1) b.append(el('span', 'qty', String(l.quantity)));
  if (l.other_ticks && l.status === 'available') b.append(el('span', 'ticked'));

  const bar = el('span', 'bar');
  const fill = el('i');
  const total = Math.max(1, l.meal_ends_at - l.first_seen_at);
  const frac = Math.max(0, Math.min(1, (l.meal_ends_at - now()) / total));
  fill.style.setProperty('transform', `scaleX(${frac})`);
  fill.style.setProperty('background', `hsl(${Math.round(frac * 120)} 75% 52%)`);
  bar.append(fill);
  b.append(bar);

  if (l.status === 'sold') b.append(el('span', 'stamp', 'SOLD'));
  if (l.status === 'closed') b.append(el('span', 'stamp', 'CLOSED'));
  b.setAttribute('aria-label', `${cap(l.mess)} ${l.meal}, ${priceText(l)}, ${l.seller.name || l.seller.phone || 'unknown seller'}`);
  return b;
}

function priceText(l) {
  if (l.kind === 'exchange' && l.price == null) return `swap for ${(l.wants || []).map(cap).join(' or ') || 'another mess'}`;
  if (l.price == null) return 'price not stated';
  if (l.price === 0) return 'free';
  return `₹${l.price}`;
}

function priceNode(l) {
  const p = el('span', 'price');
  if (l.kind === 'exchange' && l.price == null) {
    p.classList.add('swap');
    const want = (l.wants || [])[0];
    if (S.tab === 'all') {
      // the corner already shows the offered mess, so read it as "→ wanted"
      p.classList.add('compact');
      p.append(el('span', 'arrow', '⇄'));
      if (want) p.append(svgEl(messIcon(want)));
    } else {
      if (want) p.append(svgEl(messIcon(want)));
      p.append(el('span', '', 'SWAP'));
    }
    if ((l.wants || []).length > 1) p.append(el('span', 'more', `+${l.wants.length - 1}`));
  } else if (l.price == null) {
    p.classList.add('unknown');
    p.textContent = '?';
  } else if (l.price === 0) {
    p.classList.add('free');
    p.textContent = 'FREE';
  } else {
    p.append(el('span', 'r', '₹'), String(l.price));
  }
  return p;
}

// ---------- listing tooltip ----------

function title(l) {
  const meal = cap(l.meal);
  if (l.kind === 'exchange') return `${cap(l.mess)} ${meal} ⇄ ${(l.wants || []).map(cap).join(' / ') || '?'}`;
  return `${cap(l.mess)} ${meal}`;
}

function rarity(l) {
  const parts = [];
  if (l.kind === 'exchange') parts.push('EXCHANGE');
  if (l.kind === 'buy') parts.push('BUY ORDER');
  if (l.diet) parts.push(DIET_LABEL[l.diet].toUpperCase());
  parts.push(l.meal.toUpperCase());
  return { rar: parts.join(' '), c: DIET_COLOR[l.diet] || 'c7' };
}

function itemTip(l) {
  const w = window_(l.meal);
  const n = now();
  const who = l.kind === 'buy' ? 'Buyer' : 'Seller';
  const lines = [
    [{ t: title(l), c: 'nm', color: messColor(l.mess) }],
    [{ t: `${cap(dayWord(l.meal_date))}${w ? `, ${hm(w.start)}-${hm(w.end)}` : ''}`, c: 'c8' }],
    0,
    [{ t: clip(l.text, 160), c: 'lore' }],
    0,
    [{ t: `${who}: `, c: 'c7' }, { t: l.seller.name || 'unknown', c: l.seller.name ? 'cf' : 'c8' }, l.seller.saved ? { t: ' (saved)', c: 'c8' } : null],
    [{ t: 'Phone: ', c: 'c7' }, l.seller.phone ? { t: l.seller.phone, c: 'cb' } : { t: 'hidden by WhatsApp', c: 'c8' }],
  ];
  const gs = l.groups || [];
  if (gs.length === 1) lines.push([{ t: 'Group: ', c: 'c7' }, { t: shortGroup(gs[0]), c: 'cf' }]);
  else {
    lines.push([{ t: `Groups (${gs.length}):`, c: 'c7' }]);
    for (const g of gs) lines.push([{ t: '  ' + shortGroup(g), c: 'cf' }]);
  }
  lines.push([{ t: 'Posted: ', c: 'c7' }, { t: `${ago(l.first_seen_at)}, ${hm(l.first_seen_at)}`, c: 'cf' }]);
  if (l.last_seen_at - l.first_seen_at > 120) lines.push([{ t: 'Reposted: ', c: 'c7' }, { t: ago(l.last_seen_at), c: 'cf' }]);
  if (l.quantity > 1) lines.push([{ t: 'Quantity: ', c: 'c7' }, { t: `${l.quantity} registrations`, c: 'cf' }]);
  if (l.other_ticks && l.status === 'available') {
    lines.push([{ t: `${l.other_ticks} other ${l.other_ticks === 1 ? 'person' : 'people'} ticked this. It may be gone.`, c: 'c6' }]);
  }
  lines.push(0);
  if (l.kind === 'exchange') {
    lines.push([{ t: 'Wants: ', c: 'c7' }, { t: (l.wants || []).map(cap).join(' or ') || 'not said', c: 'cd' }]);
  }
  if (l.kind !== 'exchange' || l.price != null) {
    const label = l.kind === 'buy' ? 'Offering: ' : 'Buy it now: ';
    const val = l.price == null ? { t: 'ask', c: 'c8' } : l.price === 0 ? { t: 'FREE', c: 'ca' } : { t: `₹${l.price}`, c: 'c6' };
    lines.push([{ t: label, c: 'c7' }, val]);
  }
  if (l.status === 'sold') lines.push([{ t: 'Status: ', c: 'c7' }, { t: 'Sold', c: 'ca' }]);
  else if (l.status === 'closed') lines.push([{ t: 'Status: ', c: 'c7' }, { t: 'Withdrawn by seller', c: 'cc' }]);
  else lines.push([{ t: 'Ends in: ', c: 'c7' }, { t: l.meal_ends_at > n ? dur(l.meal_ends_at - n) : 'ended', c: 'cf' }]);
  lines.push(0, [{ t: 'Click to inspect!', c: 'ce' }], rarity(l));
  return lines;
}

const clip = (s, n) => (s.length > n ? s.slice(0, n - 1).trimEnd() + '…' : s);

// ---------- tooltip engine ----------

const tipEl = $('tip');
let tipFor = null;
let tipFn = null;

function fillTip(lines) {
  tipEl.replaceChildren();
  for (const ln of lines) {
    if (!ln) { tipEl.append(el('div', 'gap')); continue; }
    if (ln.rar) { tipEl.append(el('div', 'rar ' + ln.c, ln.rar)); continue; }
    const row = el('div', 'ln');
    for (const seg of ln) {
      if (!seg || !seg.t) continue;
      const s = el('span', seg.c, seg.t);
      if (seg.color) s.style.setProperty('color', seg.color);
      row.append(s);
    }
    if (row.childNodes.length) tipEl.append(row);
  }
}

function placeTip(x, y) {
  const r = tipEl.getBoundingClientRect();
  let left = x + 16, top = y - 12;
  if (left + r.width > innerWidth - 8) left = x - r.width - 16;
  if (top + r.height > innerHeight - 8) top = innerHeight - r.height - 8;
  tipEl.style.setProperty('transform', `translate(${Math.max(8, left)}px, ${Math.max(8, top)}px)`);
}

function showTip(target, fn, x, y) {
  tipFor = target;
  tipFn = fn;
  fillTip(fn());
  tipEl.hidden = false;
  placeTip(x, y);
}
function hideTip() {
  tipEl.hidden = true;
  tipFor = null;
  tipFn = null;
}

// tip attaches a hover tooltip to a static element (tabs, buttons).
// Touch taps never show tooltips; the detail view covers that.
function tip(node, fn) {
  node.addEventListener('pointerenter', (e) => { if (e.pointerType === 'mouse') showTip(node, fn, e.clientX, e.clientY); });
  node.addEventListener('pointermove', (e) => { if (tipFor === node) placeTip(e.clientX, e.clientY); });
  node.addEventListener('pointerleave', hideTip);
}

const grid = $('grid');
grid.addEventListener('pointerover', (e) => {
  if (e.pointerType !== 'mouse') return;
  const b = e.target.closest('.item');
  if (!b || b === tipFor) return;
  const l = byId.get(+b.dataset.id);
  if (l) showTip(b, () => itemTip(l), e.clientX, e.clientY);
});
grid.addEventListener('pointermove', (e) => { if (tipFor && grid.contains(tipFor)) placeTip(e.clientX, e.clientY); });
grid.addEventListener('pointerleave', hideTip);
grid.addEventListener('click', (e) => {
  const b = e.target.closest('.item');
  if (!b) return;
  const l = byId.get(+b.dataset.id);
  if (l) openListing(l);
});

// ---------- controls ----------

function renderControls() {
  const row = $('ctlRow');
  row.replaceChildren();
  const add = (node) => { row.append(node); return node; };

  const prev = add(ctl('prev', 'PREV', S.page > 0 ? 'set' : '', () => turn(-1), () => [
    [{ t: 'Previous page', c: 'nm ca' }], [{ t: `Page ${S.page + 1} of ${pages}`, c: 'c7' }]]));
  prev.classList.add('page');
  prev.disabled = S.page === 0;
  add(el('span', 'spacer'));

  add(ctl('clock', S.sort === 'recent' ? (S.recentDir === 'desc' ? 'NEWEST' : 'OLDEST') : 'RECENT', S.sort === 'recent' ? 'on' : '', sortRecent, () => [
    [{ t: 'Sort by recency', c: 'nm ca' }],
    [{ t: 'Uses the latest repost of each listing.', c: 'c8' }],
    0,
    opt('Newest first', S.sort === 'recent' && S.recentDir === 'desc'),
    opt('Oldest first', S.sort === 'recent' && S.recentDir === 'asc'),
    0,
    [{ t: S.sort === 'recent' ? 'Click to flip!' : 'Click to sort by recency!', c: 'ce' }],
  ]));

  add(ctl('coin', S.sort === 'price' ? (S.priceDir === 'asc' ? 'LOWEST' : 'HIGHEST') : 'PRICE', S.sort === 'price' ? 'on' : '', sortPrice, () => [
    [{ t: 'Sort by price', c: 'nm c6' }],
    [{ t: 'Listings without a price go last.', c: 'c8' }],
    0,
    opt('Lowest price', S.sort === 'price' && S.priceDir === 'asc'),
    opt('Highest price', S.sort === 'price' && S.priceDir === 'desc'),
    0,
    [{ t: S.sort === 'price' ? 'Click to flip!' : 'Click to sort by price!', c: 'ce' }],
  ]));

  const dietOff = S.tab !== 'all' && vegOnly(S.tab);
  const diet = add(ctl(null, dietOff ? 'VEG ONLY' : DIET_LABEL[S.diet].toUpperCase(), S.diet !== 'all' && !dietOff ? 'on' : '', cycleDiet, () => dietOff
    ? [[{ t: 'Diet filter', c: 'nm ca' }], [{ t: `${cap(S.tab)} serves only veg, so there is nothing to filter.`, c: 'c7' }]]
    : [
      [{ t: 'Diet filter', c: 'nm ca' }],
      [{ t: 'Kadamba and Bakul serve both.', c: 'c8' }],
      0,
      ...DIETS.map((d) => opt(DIET_LABEL[d], S.diet === d, DIET_COLOR[d])),
      0,
      [{ t: 'Click to cycle!', c: 'ce' }],
    ], dietMark(dietOff ? 'veg' : S.diet === 'all' ? '' : S.diet)));
  diet.disabled = dietOff;

  add(ctl(null, S.meal === 'all' ? 'ALL MEALS' : S.meal.toUpperCase(), S.meal !== 'all' ? 'on' : '', cycleMeal, () => [
    [{ t: 'Meal filter', c: 'nm ce' }],
    0,
    opt('All meals', S.meal === 'all'),
    ...MEALS.map((m) => {
      const w = window_(m);
      return opt(`${cap(m)}${w ? `  ${hm(w.start)}-${hm(w.end)}` : ''}`, S.meal === m);
    }),
    0,
    [{ t: 'Click to cycle!', c: 'ce' }],
  ], S.meal === 'all' ? mealsAllIcon() : icon(S.meal)));

  add(ctl('check', S.sold ? 'SOLD: ON' : 'SOLD: OFF', S.sold ? 'on' : '', toggleSold, () => [
    [{ t: 'Show sold and withdrawn', c: 'nm ca' }],
    [{ t: 'Sold means the seller ticked it or said so.', c: 'c8' }],
    [{ t: 'Sales agreed in DMs stay invisible.', c: 'c8' }],
    0,
    opt('Hide them', !S.sold),
    opt('Show them, greyed out', S.sold),
    0,
    [{ t: 'Click to toggle!', c: 'ce' }],
  ]));

  add(ctl(S.mode === 'sell' ? 'ingot' : 'paper', S.mode === 'sell' ? 'SELLING' : 'WANTED', 'set', toggleMode, () => [
    [{ t: 'Market side', c: 'nm c6' }],
    0,
    opt('Selling: buy it now', S.mode === 'sell'),
    opt('Wanted: buy orders', S.mode === 'buy'),
    0,
    [{ t: 'Click to switch!', c: 'ce' }],
  ]));

  add(el('span', 'spacer'));
  const next = add(ctl('next', 'NEXT', S.page < pages - 1 ? 'set' : '', () => turn(1), () => [
    [{ t: 'Next page', c: 'nm ca' }], [{ t: `Page ${S.page + 1} of ${pages}`, c: 'c7' }]]));
  next.classList.add('page');
  next.disabled = S.page >= pages - 1;

  const left = $('ctlLeft');
  if (!left.firstChild) {
    const help = el('button', 'slot ctl help');
    help.append(el('span', 'q', '?'), el('span', 'lbl', 'KEYS'));
    help.addEventListener('click', () => toast('Keys: 1-5 messes, arrows pages, R P D M S W, / search'));
    tip(help, () => [
      [{ t: 'Mess Auction House', c: 'nm c6' }],
      [{ t: 'Merged from your 4 mess groups.', c: 'c8' }],
      0,
      [{ t: '1-5 ', c: 'ce' }, { t: 'switch mess', c: 'c7' }],
      [{ t: '← → ', c: 'ce' }, { t: 'pages', c: 'c7' }],
      [{ t: 'R / P ', c: 'ce' }, { t: 'sort by recency / price', c: 'c7' }],
      [{ t: 'D / M ', c: 'ce' }, { t: 'diet / meal filter', c: 'c7' }],
      [{ t: 'S ', c: 'ce' }, { t: 'show sold', c: 'c7' }],
      [{ t: 'W ', c: 'ce' }, { t: 'selling or wanted', c: 'c7' }],
      [{ t: '/ ', c: 'ce' }, { t: 'search', c: 'c7' }],
      0,
      [{ t: 'Slot corners', c: 'c7' }],
      [{ t: '  diet mark, meal, quantity', c: 'c8' }],
      [{ t: '  bar: time left until the meal ends', c: 'c8' }],
      [{ t: '  shimmer: posted in the last 15 minutes', c: 'cd' }],
      [{ t: '  amber dot: someone else ticked it', c: 'c6' }],
    ]);
    left.append(help);
  }
}

function opt(text, on, color) {
  return [{ t: on ? '▶ ' : '   ', c: 'cb' }, { t: text, c: on ? (color || 'cb') : 'c8' }];
}

function ctl(art, label, cls, onClick, tipFn, markup) {
  const b = el('button', 'slot ctl' + (cls ? ' ' + cls : ''));
  b.append(svgEl(markup || icon(art)), el('span', 'lbl', label));
  b.addEventListener('click', () => {
    onClick();
    if (tipFor === b) {
      // keep the tooltip on the same button after re-render
      requestAnimationFrame(() => {
        const nb = [...$('ctlRow').children].find((c) => c.querySelector('.lbl') && c.dataset.k === b.dataset.k);
        if (nb) showTip(nb, tipFn, lastMouse.x, lastMouse.y);
      });
    }
  });
  b.dataset.k = art || label;
  b.setAttribute('aria-label', label);
  tip(b, tipFn);
  return b;
}
const lastMouse = { x: 0, y: 0 };
addEventListener('mousemove', (e) => { lastMouse.x = e.clientX; lastMouse.y = e.clientY; }, { passive: true });

// ---------- actions ----------

function update(fn) {
  fn();
  persist();
  hideTip();
  render();
}
const setTab = (t) => update(() => { S.tab = t; S.page = 0; });
const turn = (d) => update(() => { S.page = Math.max(0, Math.min(pages - 1, S.page + d)); });
const sortRecent = () => update(() => {
  if (S.sort === 'recent') S.recentDir = S.recentDir === 'desc' ? 'asc' : 'desc';
  S.sort = 'recent'; S.page = 0;
});
const sortPrice = () => update(() => {
  if (S.sort === 'price') S.priceDir = S.priceDir === 'asc' ? 'desc' : 'asc';
  S.sort = 'price'; S.page = 0;
});
const cycleDiet = () => update(() => { S.diet = DIETS[(DIETS.indexOf(S.diet) + 1) % DIETS.length]; S.page = 0; });
const cycleMeal = () => update(() => {
  const all = ['all', ...MEALS];
  S.meal = all[(all.indexOf(S.meal) + 1) % all.length]; S.page = 0;
});
const toggleSold = () => update(() => { S.sold = !S.sold; S.page = 0; });
const toggleMode = () => update(() => { S.mode = S.mode === 'sell' ? 'buy' : 'sell'; S.page = 0; });

$('pgPrev').addEventListener('click', () => turn(-1));
$('pgNext').addEventListener('click', () => turn(1));

const q = $('q');
q.addEventListener('input', () => { S.q = q.value.trim(); S.page = 0; render(); });

addEventListener('keydown', (e) => {
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  if (!$('modal').hidden) { if (e.key === 'Escape') closeModal(); return; }
  if (document.activeElement === q) {
    if (e.key === 'Escape') { q.value = ''; S.q = ''; q.blur(); render(); }
    return;
  }
  const k = e.key.toLowerCase();
  const names = D ? [...D.messes.map((m) => m.name), 'all'] : [];
  if (/^[1-9]$/.test(k) && names[+k - 1]) setTab(names[+k - 1]);
  else if (k === 'arrowleft') turn(-1);
  else if (k === 'arrowright') turn(1);
  else if (k === 'r') sortRecent();
  else if (k === 'p') sortPrice();
  else if (k === 'd' && !(S.tab !== 'all' && vegOnly(S.tab))) cycleDiet();
  else if (k === 'm') cycleMeal();
  else if (k === 's') toggleSold();
  else if (k === 'w') toggleMode();
  else if (k === '/') { e.preventDefault(); q.focus(); }
  else return;
});

// ---------- detail view ----------

const modal = $('modal');
const sheet = $('sheet');
modal.addEventListener('click', (e) => { if (e.target === modal) closeModal(); });
function closeModal() { modal.hidden = true; sheet.replaceChildren(); }

function digits(phone) { return (phone || '').replace(/\D/g, ''); }

function openListing(l) {
  hideTip();
  sheet.replaceChildren();
  const head = el('div', 'sheet-head');
  const big = slotNode(l);
  big.classList.remove('pop', 'item');
  big.disabled = true;
  head.append(big);
  const info = el('div');
  const h = el('h2', '', title(l));
  h.style.setProperty('color', messColor(l.mess));
  const w = window_(l.meal);
  info.append(h, el('div', 'sub', `${cap(dayWord(l.meal_date))}${w ? `, ${hm(w.start)}-${hm(w.end)}` : ''} · ${priceText(l)}`));
  const r = rarity(l);
  const pill = el('span', 'pill ' + (l.status === 'available' ? r.c : l.status === 'sold' ? 'ca' : 'c8'),
    l.status === 'available' ? r.rar : l.status === 'sold' ? 'SOLD' : 'WITHDRAWN');
  info.append(pill);
  head.append(info);
  sheet.append(head);

  const facts = el('dl', 'facts');
  const fact = (k, v, cls) => { facts.append(el('dt', '', k)); facts.append(el('dd', cls || '', v)); };
  fact(l.kind === 'buy' ? 'Buyer' : 'Seller', l.seller.name || 'unknown');
  fact('Phone', l.seller.phone || 'hidden by WhatsApp in this group', l.seller.phone ? 'cb' : 'c8');
  if (l.kind === 'exchange') fact('Wants', (l.wants || []).map(cap).join(' or ') || 'not said', 'cd');
  fact('Diet', l.diet ? DIET_LABEL[l.diet] : 'not stated', DIET_COLOR[l.diet] || 'c8');
  if (l.quantity > 1) fact('Quantity', `${l.quantity} registrations`);
  fact('Posted', `${ago(l.first_seen_at)} (${hm(l.first_seen_at)})`);
  if (l.last_seen_at - l.first_seen_at > 120) fact('Last repost', `${ago(l.last_seen_at)} (${hm(l.last_seen_at)})`);
  fact('Meal ends', l.meal_ends_at > now() ? `in ${dur(l.meal_ends_at - now())} (${hm(l.meal_ends_at)})` : 'ended');
  if (l.other_ticks) fact('Others ticked', `${l.other_ticks}. It may already be gone.`, 'c6');
  if (l.meal_source === 'time') fact('Meal from', 'when it was posted. The post did not name a meal.', 'c8');
  sheet.append(facts);

  const copies = el('div', 'copies');
  copies.append(el('h3', '', 'POSTED IN'));
  for (const g of l.groups || []) {
    const c = el('div', 'copy');
    const meta = el('div', 'copy-meta');
    meta.append(el('b', '', shortGroup(g)));
    c.append(meta, el('div', 'copy-text', l.text));
    copies.append(c);
  }
  sheet.append(copies);
  fetch(`api/listing?id=${encodeURIComponent(l.id)}`, { cache: 'no-store' })
    .then((r) => (r.ok ? r.json() : null))
    .then((d) => {
      if (!d || !d.copies || modal.hidden) return;
      copies.replaceChildren(el('h3', '', `POSTED ${d.copies.length} ${d.copies.length === 1 ? 'TIME' : 'TIMES'}`));
      for (const cp of d.copies) {
        const c = el('div', 'copy' + (cp.deleted ? ' deleted' : ''));
        const meta = el('div', 'copy-meta');
        meta.append(el('b', '', shortGroup(cp.group)), el('span', '', cp.time.slice(11, 16)));
        if (cp.edited) meta.append(el('span', '', 'edited'));
        if (cp.deleted) meta.append(el('span', '', 'deleted'));
        c.append(meta, el('div', 'copy-text', cp.text));
        copies.append(c);
      }
    })
    .catch(() => {});

  const actions = el('div', 'actions');
  const ph = digits(l.seller.phone);
  const wa = el('a', 'btn primary');
  wa.append(svgEl(waIcon), l.kind === 'buy' ? 'Message buyer' : 'Message seller');
  if (ph) {
    wa.href = `https://wa.me/${ph}?text=${encodeURIComponent(greeting(l))}`;
    wa.target = '_blank';
    wa.rel = 'noopener noreferrer';
  } else {
    wa.setAttribute('aria-disabled', 'true');
  }
  actions.append(wa);
  if (ph) {
    const copy = el('button', 'btn', 'Copy number');
    copy.addEventListener('click', () => {
      navigator.clipboard?.writeText(l.seller.phone).then(() => toast('Number copied'), () => toast(l.seller.phone));
    });
    actions.append(copy);
  }
  const close = el('button', 'btn', 'Close');
  close.addEventListener('click', closeModal);
  actions.append(close);
  sheet.append(actions);
  sheet.append(el('div', 'note', ph
    ? 'The message button opens WhatsApp with a draft. Nothing is sent until you send it.'
    : 'WhatsApp hides this member\'s number. Reply to their post in the group instead.'));
  modal.hidden = false;
}

function greeting(l) {
  const meal = `${cap(l.mess)} ${l.meal}`;
  if (l.kind === 'buy') return `Hi, I saw you're looking for ${meal}. I have one, still need it?`;
  if (l.kind === 'exchange') return `Hi, about your ${meal} exchange, is it still available?`;
  return `Hi, is your ${meal} still available?`;
}

let toastTimer = 0;
function toast(msg) {
  const t = $('toast');
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, 2200);
}

// ---------- live status ----------

function tickLive() {
  const live = $('live');
  const s = Math.round((Date.now() - lastOK) / 1000);
  live.classList.toggle('off', failed || !lastOK);
  $('liveText').textContent = !lastOK ? 'connecting' : failed ? `offline, last update ${s}s ago` : streaming || s <= pollEvery() + 2 ? 'live' : `updated ${s}s ago`;
  if (tipFor && tipFn && !tipEl.hidden && document.body.contains(tipFor)) fillTip(tipFn());
}

// ---------- boot ----------

$('logo').append(svgEl(icon('coin')));
layout();
addEventListener('resize', () => {
  const before = `${L.cols}x${L.rows}x${L.s}`;
  layout();
  if (`${L.cols}x${L.rows}x${L.s}` !== before) render();
});
load();
connect();
setInterval(tickLive, 1000);
setInterval(() => { if (D && document.visibilityState === 'visible') { renderDay(); renderGrid(); } }, 30000);
document.addEventListener('visibilitychange', () => { if (document.visibilityState === 'visible') load(); });
