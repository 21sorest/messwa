'use strict';

// Shared by the board (app.js) and the sales page (sales.js): DOM helpers,
// pixel art and mess identities.

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
  chart: [[
    '............',
    '..........bb',
    '..........bb',
    '....yy....bb',
    '....yy....bb',
    '....yy....bb',
    '.kk.yy....bb',
    '.kk.yy....bb',
    '.kk.yy.pp.bb',
    '.kk.yy.pp.bb',
    '.kk.yy.pp.bb',
    '.kk.yy.pp.bb',
  ], { k: '#f7c548', y: '#36c9b0', p: '#ff5a36', b: '#b89cff' }],
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

// ---------- tooltip engine ----------

const tipEl = $('tip');
const lastMouse = { x: -1, y: -1 };
addEventListener('mousemove', (e) => { lastMouse.x = e.clientX; lastMouse.y = e.clientY; }, { passive: true });
let tipFor = null;
let tipFn = null;

// fillLines writes tooltip lines into box: segments of {t, c, color}, a
// falsy line for a gap, or {rar, c} for the closing rarity line.
function fillLines(box, lines) {
  box.replaceChildren();
  for (const ln of lines) {
    if (!ln) { box.append(el('div', 'gap')); continue; }
    if (ln.rar) {
      const r = el('div', 'rar ' + (ln.c || ''), ln.rar);
      if (ln.color) r.style.setProperty('color', ln.color);
      box.append(r);
      continue;
    }
    const row = el('div', 'ln');
    for (const seg of ln) {
      if (!seg || !seg.t) continue;
      const s = el('span', seg.c, seg.t);
      if (seg.color) s.style.setProperty('color', seg.color);
      row.append(s);
    }
    if (row.childNodes.length) box.append(row);
  }
}
const fillTip = (lines) => fillLines(tipEl, lines);

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
  node._tip = fn;
  node.addEventListener('pointerenter', (e) => { if (e.pointerType === 'mouse') showTip(node, fn, e.clientX, e.clientY); });
  node.addEventListener('pointermove', (e) => { if (tipFor === node) placeTip(e.clientX, e.clientY); });
  node.addEventListener('pointerleave', hideTip);
}

// ---------- money ----------

// rupees prints whole amounts bare and paise with two decimals: ₹60, ₹2.37.
const rupees = (v) => '₹' + (Number.isInteger(v) ? v.toLocaleString('en-IN') : v.toFixed(2));
