// Run with node --test internal/dashboard/ledger_test.cjs.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const NOW = 1794800000; // Sun 15 Nov 2026, before every meal below
const lunch = (date, extra = {}) => {
  const start = Math.floor(new Date(date + 'T06:00:00Z').getTime() / 1000);
  return { meal: 'lunch', state: 'open', mess: 'palash', start, end: start + 7200, ask: 60, ...extra };
};

function ledger(send = async () => ({})) {
  class Element {
    constructor() { this.children = []; this.dataset = {}; this.classList = { toggle() {}, add() {}, remove() {}, contains() { return false; } }; this.style = { setProperty() {} }; }
    append(...items) { this.children.push(...items); }
    prepend(item) { this.children.unshift(item); }
    replaceChildren(...items) { this.children = items; }
    setAttribute() {}
    removeAttribute() {}
    focus() {}
    addEventListener() {}
    scrollIntoView() {}
    querySelector() { return null; }
    querySelectorAll() { return []; }
    get childNodes() { return this.children; }
  }
  const nodes = new Map();
  const addDays = (date, n) => { const d = new Date(date + 'T12:00:00Z'); d.setUTCDate(d.getUTCDate() + n); return d.toISOString().slice(0, 10); };
  const toasts = [];
  const ctx = vm.createContext({
    D: {
      owner: true, today: '2026-11-16', list_at: -900, sales: [],
      days: [{ date: '2026-11-15', cells: [lunch('2026-11-15', { state: 'ate', end: NOW - 1 })] }, { date: '2026-11-16', cells: [lunch('2026-11-16')] }],
      upcoming: [{ date: '2026-11-16', cells: [lunch('2026-11-16')] }, { date: '2026-11-17', cells: [lunch('2026-11-17')] }, { date: '2026-11-18', cells: [lunch('2026-11-18')] }],
    },
    $: (id) => { if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id); },
    el: (tag, cls, text) => { const e = new Element(); e.tag = tag; e.className = cls; e.textContent = text; return e; },
    document: { querySelector: () => null }, addEventListener() {},
    addDays, dayOf: (d) => new Date(d + 'T12:00:00Z'), fmt: (s) => s, fmtDay: (s) => s, fmtShort: (s) => s,
    sum: (xs) => xs.reduce((a, b) => a + b, 0), rupees: (v) => `₹${v}`, isSold: () => false, meals: () => ['lunch'],
    cellNode: () => new Element(), cellLines: () => [], tip() {}, tipFor: null, hideTip() {}, fillLines(box) { box.replaceChildren(); },
    svgEl: () => new Element(), icon: () => '', cap: (s) => s, hm: () => '12:00', now: () => NOW, messName: (s) => s, messColor: () => '',
    DIET_LABEL: { veg: 'Veg' }, signedPct: (v) => `${v}%`,
    sellDefaults: () => [40, 80], sellBounds: () => ({ lo: 1, hi: 150 }), priceText: () => '',
    priceSlider: () => ({ nodes: [], get: () => [40, 80] }),
    send, load: async () => {}, toast: (t) => toasts.push(t), openSell() {}, SELL: { isSold: () => false },
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, 'static/ledger.js'), 'utf8'), ctx);
  const run = (code) => vm.runInContext(code, ctx);
  run('renderCalendar()');
  return { run, ctx, toasts };
}

test('the calendar opens on the Monday of this week and reaches four weeks either side', () => {
  const p = ledger();
  assert.equal(p.run('calWeek'), '2026-11-16');
  assert.equal(p.run('mondayOf("2026-11-22")'), '2026-11-16');
  assert.equal(p.run('mondayOf("2026-11-15")'), '2026-11-09');
  assert.equal(p.run('calFirst()'), '2026-11-09');
});

test('a plain click opens one meal; clicking it again closes it', () => {
  const p = ledger();
  p.run("calClick('2026-11-17|lunch', false)");
  assert.deepEqual([...p.run('calPicked')], ['2026-11-17|lunch']);
  p.run("calClick('2026-11-18|lunch', false)");
  assert.deepEqual([...p.run('calPicked')], ['2026-11-18|lunch']);
  p.run("calClick('2026-11-18|lunch', false)");
  assert.equal(p.run('calPicked.length'), 0);
});

test('adding picks several meals still to come, but never a past or running one', () => {
  const p = ledger();
  p.run("D.upcoming[2].cells[0] = { ...D.upcoming[2].cells[0], state: 'selling', task: { id: 8, min: 30, max: 30 } }");
  p.run("calClick('2026-11-15|lunch', false)");
  p.run("calClick('2026-11-16|lunch', true)");
  assert.deepEqual([...p.run('calPicked')], ['2026-11-16|lunch'], 'the past meal leaves the pick');
  p.run("calClick('2026-11-17|lunch', true)");
  p.run("calClick('2026-11-18|lunch', true)");
  assert.deepEqual([...p.run('calPicked')], ['2026-11-18|lunch'], 'a running sale opens on its own');
  p.run("calClick('2026-11-16|lunch', true); calClick('2026-11-17|lunch', true); calClick('2026-11-16|lunch', true)");
  assert.deepEqual([...p.run('calPicked')], ['2026-11-17|lunch'], 'adding again takes it out');
});

test('a heading picks its meals, and picks them out again when they are all in', () => {
  const p = ledger();
  const keys = "['2026-11-16|lunch', '2026-11-17|lunch', '2026-11-18|lunch']";
  p.run(`calPickAll(${keys})`);
  assert.equal(p.run('calPicked.length'), 3);
  p.run(`calPickAll(${keys})`);
  assert.equal(p.run('calPicked.length'), 0);
});

test('editing a waiting schedule keeps its id and the revision the page read', () => {
  const p = ledger();
  const c = p.run("D.upcoming[1].cells[0]");
  c.state = 'scheduled';
  c.schedule = { id: 12, revision: 3, state: 'waiting', min: 40, max: 80, at: c.start - 900, autopilot: false };
  p.run("calPicked = ['2026-11-17|lunch']; calForm = null; renderCalendar()");
  const f = p.run('calForm');
  assert.deepEqual([f.min, f.max, f.off, f.autopilot], [40, 80, -15, false], 'the form starts from the saved schedule');
  assert.equal(p.run('calUnchanged(D.upcoming[1].cells[0], calForm)'), true);
  f.min = 30;
  const body = p.run("calPlanBody('2026-11-17', D.upcoming[1].cells[0], calForm)");
  assert.deepEqual([body.action, body.id, body.revision, body.min, body.at], ['plan', 12, 3, 30, c.start - 900]);
});

test('posting times stay inside what the server takes', () => {
  const p = ledger();
  const c = p.run('D.upcoming[1].cells[0]');
  assert.equal(p.run('calPlanBody("2026-11-17", D.upcoming[1].cells[0], { min: 40, max: 80, off: -60 }).at'), c.start - 1800);
  assert.equal(p.run('calPlanBody("2026-11-17", D.upcoming[1].cells[0], { min: 40, max: 80, off: 200 }).at'), c.start + 90 * 60);
  c.end = c.start + 3600;
  assert.equal(p.run('calPlanBody("2026-11-17", D.upcoming[1].cells[0], { min: 40, max: 80, off: 90 }).at'), c.start + 59 * 60);
  // An offer can't go up while the meal before is still being served.
  c.opens = c.start + 600;
  assert.equal(p.run('calAt(D.upcoming[1].cells[0], -15)'), c.start + 600);
});

test('invalid prices send nothing', async () => {
  let sent = 0;
  const p = ledger(async () => { sent++; return {}; });
  p.run("calPicked = ['2026-11-17|lunch', '2026-11-18|lunch']; calForm = null; renderCalendar(); calForm.min = 90; calForm.max = 50");
  await p.run('calSave(calPicks())');
  assert.equal(sent, 0);
});

test('saving several meals sends one plan each; failures stay picked with the reason', async () => {
  const bodies = [];
  const p = ledger(async (_m, _url, body) => { bodies.push(body); if (body.date === '2026-11-17') throw new Error('not registered'); return { at: body.at }; });
  p.run("calPicked = ['2026-11-16|lunch', '2026-11-17|lunch', '2026-11-18|lunch']; calForm = null; renderCalendar()");
  p.run('calForm.min = 35; calForm.max = 70; calForm.autopilot = true; calForm.touched = true');
  await p.run('calSave(calPicks())');
  assert.equal(bodies.length, 3);
  assert.ok(bodies.every((b) => b.action === 'plan' && b.id === 0 && b.min === 35 && b.max === 70 && b.autopilot === true));
  assert.deepEqual([...p.run('calPicked')], ['2026-11-17|lunch']);
  assert.match(p.run('calNote.error'), /Saved 2\. .*not registered/s);
  assert.equal(p.run('calForm.min'), 35, 'the form keeps what was typed for the retry');
  assert.equal(p.run('calBusy'), false);
});

test('a pick that all saves clears and says how many', async () => {
  const p = ledger(async (_m, _u, body) => ({ at: body.at }));
  p.run("calPicked = ['2026-11-17|lunch', '2026-11-18|lunch']; calForm = null; renderCalendar()");
  await p.run('calSave(calPicks())');
  assert.equal(p.run('calPicked.length'), 0);
  assert.deepEqual(p.toasts, ['Scheduled 2 meals.']);
});

test('meals already set to the form are skipped', async () => {
  const bodies = [];
  const p = ledger(async (_m, _u, body) => { bodies.push(body); return { at: body.at }; });
  const c = p.run('D.upcoming[1].cells[0]');
  c.state = 'scheduled';
  c.schedule = { id: 12, revision: 3, state: 'waiting', min: 40, max: 80, at: c.start - 900, autopilot: false };
  p.run("calPicked = ['2026-11-17|lunch', '2026-11-18|lunch']; calForm = null; renderCalendar()");
  await p.run('calSave(calPicks())');
  assert.deepEqual(bodies.map((b) => b.date), ['2026-11-18']);
});

test('the form follows a reload until you change it', () => {
  const p = ledger();
  const c = p.run('D.upcoming[1].cells[0]');
  c.state = 'scheduled';
  c.schedule = { id: 12, revision: 3, state: 'waiting', min: 40, max: 80, at: c.start - 900 };
  p.run("calPicked = ['2026-11-17|lunch']; calForm = null; renderCalendar()");
  c.schedule = { ...c.schedule, revision: 4, min: 45 };
  p.run('renderCalendar()');
  assert.equal(p.run('calForm.min'), 45);
  p.run('calForm.min = 30; calForm.touched = true');
  c.schedule = { ...c.schedule, revision: 5, min: 50 };
  p.run('renderCalendar()');
  assert.equal(p.run('calForm.min'), 30);
});

test('a kept meal can be handed back to recurring rules', async () => {
  const sent = [];
  const p = ledger(async (_m, url, body) => { sent.push([url, body]); return {}; });
  p.run("D.upcoming[1].cells[0].kept = true; calPicked = ['2026-11-17|lunch']; calForm = null; renderCalendar()");
  const all = (e) => [e, ...(e.children || []).flatMap(all)];
  const allow = all(p.ctx.$('insp')).find((e) => e.textContent === 'ALLOW RULES AGAIN');
  assert.ok(allow);
  await allow.onclick();
  assert.deepEqual(JSON.parse(JSON.stringify(sent)), [['api/recurring-rules', { action: 'allow', date: '2026-11-17', meal: 'lunch' }]]);
});
