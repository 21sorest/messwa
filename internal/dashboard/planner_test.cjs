// Run with node --test internal/dashboard/planner_test.cjs.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

function planner(send = async () => ({})) {
  class Element {
    constructor() { this.children = []; this.dataset = {}; this.classList = { toggle() {}, add() {}, remove() {} }; this.style = { setProperty() {} }; }
    append(...items) { this.children.push(...items); }
    after() {}
    prepend(item) { this.children.unshift(item); }
    replaceChildren(...items) { this.children = items; }
    setAttribute() {}
    removeAttribute() {}
    focus() {}
    addEventListener() {}
    contains() { return false; }
    querySelectorAll() { return []; }
  }
  const nodes = new Map();
  const c = { meal: 'lunch', state: 'open', sellable: true, mess: 'palash', start: 1794891600, end: 1794898800 };
  const ctx = vm.createContext({
    D: { owner: true, today: '2026-11-16', list_at: -900, upcoming: [{ date: '2026-11-16', cells: [c] }, { date: '2026-11-17', cells: [{ ...c }] }] },
    $: (id) => { if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id); },
    el: (tag,cls,text) => { const e=new Element();e.tag=tag;e.textContent=text;return e; }, document: { activeElement: null }, window: { addEventListener() {} },
    sellDefaults: () => [40, 80], sellBounds: () => ({}), meals: () => ['lunch'],
    cap: (s) => s, messName: (s) => s, fmtDay: (s) => s, fmt: (s) => s, send,
    svgEl: () => new Element(), icon: () => '', messColor: () => '', messIcon: () => '',
    priceSlider: () => ({ nodes: [], get: () => [40, 80] }),
    now: () => 1794800000, dayOf: (d) => new Date(d + 'T12:00:00Z'), load: async () => {}, openSell() {}, priceText: () => '', hm: () => '',
    cellNode: () => new Element(), rupees: (v) => `₹${v}`, DIET_LABEL: { veg: 'Veg' }, confirm: () => true,
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, 'static/planner.js'), 'utf8'), ctx);
  return { run: (code) => vm.runInContext(code, ctx), nodes, c, ctx };
}

test('select and deselect registered meals; never cancel an active sale', () => {
  const p = planner();
  p.run("D.upcoming[0].cells.push({ meal: 'dinner', state: 'selling', task: { id: 8 } }, { meal: 'breakfast', state: 'none' })");
  p.run('planSelect(D.upcoming, true)');
  assert.equal(p.run('planActions().length'), 2);
  assert.equal(p.run('planActions()[0].body.min'), 40);
  p.run('planSelect(D.upcoming, false)');
  assert.equal(p.run('planActions().length'), 0);
});

test('unchecking a saved schedule produces an unschedule action only', () => {
  const p = planner();
  p.c.schedule = { id: 12, revision: 2, min: 40, max: 80, at: 1794890700 };
  p.run('planSelect(D.upcoming, false)');
  assert.equal(p.run('planActions().length'), 1);
  assert.equal(p.run('planActions()[0].body.action'), 'unplan');
  assert.equal(p.run('planActions()[0].body.id'), 12);
});

test('invalid prices prevent every request in a bulk save', async () => {
  let sent = 0;
  const p = planner(async () => { sent++; });
  p.run('planSelect(D.upcoming, true); planDraft(D.upcoming[1].date, D.upcoming[1].cells[0]).min = 90');
  await p.run("$('planSave').onclick()");
  assert.equal(sent, 0);
  assert.match(p.ctx.$('planError').textContent, /whole rupees/);
});

test('partial failure retains only the remaining draft for retry', async () => {
  let sent = 0;
  const p = planner(async () => { if (++sent === 2) throw new Error('connection lost'); });
  p.run('planSelect(D.upcoming, true)');
  await p.run("$('planSave').onclick()");
  assert.equal(sent, 2);
  assert.equal(p.run('planDrafts.size'), 1);
  assert.equal(p.run('planActions()[0].body.date'), '2026-11-17');
  assert.match(p.ctx.$('planError').textContent, /could not be saved/);
  assert.match(p.run("planResults.get(\'2026-11-17|lunch\').error"), /connection lost/);
  assert.equal(p.run('planBusy'), false);
});

test('editing a waiting schedule keeps its identity and frozen posting time', () => {
 const p=planner(); p.c.schedule={id:12,revision:3,min:40,max:80,at:1794890700,autopilot:false};
 p.run('planDraft(D.upcoming[0].date,D.upcoming[0].cells[0]).min=30');
 assert.equal(p.run('planActions()[0].body.action'),'plan');
 assert.equal(p.run('planActions()[0].body.id'),12);
 assert.equal(p.run('planActions()[0].body.revision'),3);
 assert.equal(p.run('planActions()[0].body.at'),1794890700);
 p.c.schedule.revision=4;
 assert.equal(p.run('planActions()[0].body.revision'),3);
});
test('shared settings apply only to selected meals and preserve unselected overrides', () => {
 const p=planner(); p.run('planSelect([D.upcoming[0]],true)');
 p.ctx.$('planMin').value='30';p.ctx.$('planMax').value='60';p.ctx.$('planBefore').value='20';p.ctx.$('planAuto').checked=true;
 p.run('planApply(D.upcoming)');
 assert.equal(p.run('planActions().length'),1);
 assert.equal(p.run('planActions()[0].body.min'),30);
 assert.equal(p.run('planActions()[0].body.max'),60);
 assert.equal(p.run('planActions()[0].body.at'),p.c.start-1200);
 assert.equal(p.run('planActions()[0].body.autopilot'),true);
});
test('a failed meal does not prevent later saves, and retry sends only that meal', async () => {
 let count=0;const dates=[];
 const p=planner(async (_method,_path,body)=>{dates.push(body.date);if(++count===2)throw new Error('offline');return {id:3,at:body.at};});
 p.run("D.upcoming.push({date:'2026-11-18',cells:[{...D.upcoming[0].cells[0]}]});planSelect(D.upcoming,true)");
 await p.run("$('planSave').onclick()"); assert.equal(count,3);assert.equal(p.run('planDrafts.size'),1);
 await p.run("$('planSave').onclick()");assert.equal(count,4);assert.equal(dates.at(-1),'2026-11-17');assert.equal(p.run('planDrafts.size'),0);
});
test('range selection supports weekday lunches without selecting other meals',()=>{
 const p=planner();p.run("D.upcoming.push({date:'2026-11-21',cells:[{...D.upcoming[0].cells[0]}]});D.upcoming[0].cells.push({...D.upcoming[0].cells[0],meal:'dinner'});renderPlanner()");
 p.ctx.$('planFrom').value='2026-11-16';p.ctx.$('planTo').value='2026-11-21';p.ctx.$('planMeal').value='lunch';p.ctx.$('planWeekdays').checked=true;
 p.run("$('planRangeSelect').onclick()");assert.equal(p.run('planActions().length'),2);assert.equal(p.run("planActions().every(a=>a.body.meal==='lunch')"),true);
});

test('reviewing a changed schedule retains draft settings and adopts the current revision',()=>{
 const p=planner();p.c.schedule={id:12,revision:3,min:40,max:80,at:1794890700,autopilot:false};
 p.run("planPicked='2026-11-16|lunch';planDraft('2026-11-16',D.upcoming[0].cells[0]).min=30");p.c.schedule.revision=4;p.run('renderPlanInspector()');
 const review=p.ctx.$('planInspector').children.flatMap(e=>[e,...(e.children||[])]).find(e=>e.textContent==='USE CURRENT SCHEDULE');assert.ok(review);review.onclick();
 assert.equal(p.run('planActions()[0].body.revision'),4);assert.equal(p.run('planActions()[0].body.min'),30);
});

test('clicking a meal selects it and opens it; clicking the open meal again keeps it', () => {
  const p = planner();
  const key = "planKey(D.upcoming[1].date, D.upcoming[1].cells[0])";
  p.run(`planClick(${key})`);
  assert.equal(p.run('planPicked'), '2026-11-17|lunch');
  assert.equal(p.run('planActions().length'), 1);
  p.run(`planClick(${key})`);
  assert.equal(p.run('planActions().length'), 0);
  p.run("D.upcoming[0].cells.push({ meal: 'dinner', state: 'selling', task: { id: 8, min: 30, max: 30 } })");
  p.run("planClick('2026-11-16|dinner')");
  assert.equal(p.run('planPicked'), '2026-11-16|dinner');
  assert.equal(p.run('planActions().length'), 0);
});

test('kept meals get a stamp and can be handed back to recurring rules', async () => {
  const sent = [];
  const p = planner(async (_m, url, body) => { sent.push([url, body]); return {}; });
  p.c.kept = true;
  const cell = p.run("planCellNode('2026-11-16', D.upcoming[0].cells[0])");
  assert.ok(cell.children.some((e) => e.textContent === 'KEPT'));
  p.run("planPicked='2026-11-16|lunch';renderPlanInspector()");
  const all = p.ctx.$('planInspector').children.flatMap((e) => [e, ...(e.children || [])]);
  const allow = all.find((e) => e.textContent === 'ALLOW RULES AGAIN');
  assert.ok(allow);
  await allow.onclick();
  assert.equal(JSON.stringify(sent), JSON.stringify([['api/recurring-rules', { action: 'allow', date: '2026-11-16', meal: 'lunch' }]]));
});

test('removing a saved schedule warns that recurring rules will skip the meal', () => {
  const p = planner();
  p.run("var recurringRules = [{ id: 2, name: 'Weekday lunch', enabled: true }]");
  p.c.schedule = { id: 12, revision: 3, min: 40, max: 80, at: 1794890700, rule_id: 2 };
  p.run("planPicked='2026-11-16|lunch';planDraft('2026-11-16',D.upcoming[0].cells[0]).selected=false;renderPlanInspector()");
  const text = p.ctx.$('planInspector').children.map((e) => e.textContent || '').join(' ');
  assert.match(text, /Recurring rules will skip it too/);
});
