'use strict';

// The sale planner: the next four weeks of registered meals, laid out like
// the sales ledger (days across, meals down) with the same slots. Clicking
// a meal marks it to sell and opens its settings beside the grid; nothing
// is sent until Save.

let planPage = 0;
let planBusy = false;
const planDrafts = new Map();
const planResults = new Map();
let planPicked = '';
const planKey = (date, c) => `${date}|${c.meal}`;
const planEligible = (c) => !c.task && (!c.schedule || !c.schedule.state || c.schedule.state === 'waiting') && (c.schedule || (c.state === 'open' && c.sellable));
const planDays = () => (D.upcoming || []).slice(planPage * 7, planPage * 7 + 7);
const planSelected = (date, c) => planDrafts.get(planKey(date, c))?.selected ?? !!c.schedule;
const planLocked = () => planBusy || !!D.sell_off;

function planDraft(date, c) {
  const key = planKey(date, c);
  if (!planDrafts.has(key)) {
    const [min, max] = sellDefaults(c, sellBounds(c));
    planDrafts.set(key, { selected: !!c.schedule, min, max, at: c.schedule?.at ?? c.start + (D.list_at || 0), autopilot: !!c.schedule?.autopilot, base: c.schedule ? { ...c.schedule } : null });
  }
  return planDrafts.get(key);
}

// Waiting schedules are edited in place against the revision originally read.
function planAction(date, c, draft) {
  if (!planEligible(c)) return null;
  const base = draft.base;
  if (!draft.selected) return base ? { action: 'unplan', id: base.id, revision: base.revision } : null;
  if (base && draft.min === base.min && draft.max === base.max && draft.at === base.at && draft.autopilot === !!base.autopilot) return null;
  return { action: 'plan', id: base?.id || 0, revision: base?.revision || 0, date, meal: c.meal, min: draft.min, max: draft.max, at: draft.at, autopilot: draft.autopilot };
}

function planActions() {
  return (D.upcoming || []).flatMap((day) => day.cells.flatMap((c) => {
    const draft = planDrafts.get(planKey(day.date, c));
    const action = draft && planAction(day.date, c, draft);
    return action ? [{ key: planKey(day.date, c), body: action }] : [];
  }));
}

// planChange says what Save would do to one meal: 'new', 'edit', 'drop' or ''.
function planChange(date, c) {
  const draft = planDrafts.get(planKey(date, c));
  const a = draft && planAction(date, c, draft);
  return !a ? '' : a.action === 'unplan' ? 'drop' : a.id ? 'edit' : 'new';
}

function planStatus() {
  const actions = planActions(), starts = actions.filter((a) => a.body.action === 'plan' && !a.body.id);
  const immediate = starts.filter((a) => a.body.at <= now()).length;
  const updates = actions.filter((a) => a.body.action === 'plan' && a.body.id).length;
  const drops = actions.filter((a) => a.body.action === 'unplan').length;
  const parts = [starts.length && `${starts.length} new`, updates && `${updates} edited`, drops && `${drops} removed`].filter(Boolean);
  $('planStatus').textContent = planBusy ? 'Saving...'
    : actions.length ? parts.join(', ') + (immediate ? ` · ${immediate} start${immediate === 1 ? 's' : ''} selling on save` : '') : 'No unsaved changes';
  $('planSave').disabled = planBusy || !actions.length || !!D.sell_off;
  $('planReset').disabled = planBusy || !planDrafts.size;
  $('planBar').classList.toggle('dirty', actions.length > 0);
}

function planSelect(days, selected) {
  for (const day of days) for (const c of day.cells) {
    if (planEligible(c)) planDraft(day.date, c).selected = selected;
  }
  renderPlanner();
}

// planToggle selects the eligible meals in cells, or clears them when all
// already are.
function planToggle(pairs) {
  const ok = pairs.filter(([, c]) => planEligible(c));
  const on = !ok.every(([d, c]) => planSelected(d, c));
  for (const [d, c] of ok) planDraft(d, c).selected = on;
  renderPlanner();
}

// planView is the cell as Save would leave it, for the ledger's cellNode.
function planView(date, c) {
  if (!planEligible(c)) return c;
  const draft = planDrafts.get(planKey(date, c));
  if (!planSelected(date, c)) return { ...c, state: 'open', schedule: null };
  const s = draft || c.schedule;
  return { ...c, state: 'scheduled', schedule: { ...c.schedule, min: s.min, max: s.max, at: s.at, autopilot: !!s.autopilot } };
}

function planFind(key) {
  for (const day of D.upcoming || []) for (const c of day.cells) if (planKey(day.date, c) === key) return [day.date, c];
  return null;
}

// Clicking an unticked meal ticks it and opens its settings; clicking the
// open one again unticks it. Running and past meals only open.
function planClick(key) {
  const hit = planFind(key);
  if (!hit) return;
  const [date, c] = hit;
  if (planEligible(c) && !planLocked()) {
    const draft = planDraft(date, c);
    if (!draft.selected) draft.selected = true;
    else if (planPicked === key) draft.selected = false;
  }
  planPicked = key;
  renderPlanner();
}

function renderPlanner() {
  $('planner').hidden = !D.owner;
  if (!D.owner) return;
  if (typeof refreshRecurringRulesView === 'function') refreshRecurringRulesView();
  for (const day of D.upcoming || []) for (const c of day.cells) {
    const key = planKey(day.date, c), draft = planDrafts.get(key);
    if (!draft) continue;
    if (c.schedule && draft.selected && c.schedule.min === draft.min && c.schedule.max === draft.max && c.schedule.at === draft.at && !!c.schedule.autopilot === draft.autopilot) {
      planDrafts.delete(key); planResults.set(key, { message: 'Scheduled' });
    } else if (!planEligible(c) && (draft.selected || draft.base)) {
      planResults.set(key, { error: 'This meal changed or started elsewhere. Review it; your other drafts are kept.' });
    }
  }
  const days = planDays();
  if (!days.some((d) => d.cells.some((c) => planKey(d.date, c) === planPicked))) planPicked = '';
  const locked = planLocked();
  $('planSharedFields').disabled = locked;
  $('planPrev').disabled = planBusy || planPage === 0;
  $('planNext').disabled = planBusy || (planPage + 1) * 7 >= (D.upcoming || []).length;
  for (const id of ['planWeek', 'planClearWeek', 'planAll', 'planClear', 'planRangeSelect', 'planApply']) $(id).disabled = locked;
  $('planFrom').min = $('planTo').min = D.today;
  $('planFrom').max = $('planTo').max = (D.upcoming || []).at(-1)?.date || D.today;
  if (!$('planFrom').value) $('planFrom').value = D.today;
  if (!$('planTo').value) $('planTo').value = days.at(-1)?.date || D.today;
  if (!$('planBefore').value) $('planBefore').value = String(-(D.list_at || 0) / 60);
  renderPlanGrid();
  // Rebuilding the settings would drop the field being typed in.
  const typing = !planBusy && planPicked && $('planInspector').contains(document.activeElement) && document.activeElement.matches('input');
  if (!typing) renderPlanInspector();
  $('planError').textContent = D.sell_off || '';
  planStatus();
}

function renderPlanGrid() {
  const days = planDays(), locked = planLocked();
  const all = (D.upcoming || []).flatMap((d) => d.cells.map((c) => [d.date, c]));
  const scheduled = all.filter(([, c]) => c.schedule).length;
  const pending = planActions().length;
  const meta = $('planRange');
  meta.replaceChildren(days.length ? `${fmtDay(days[0].date)} - ${fmtDay(days[days.length - 1].date)}` : '');
  if (scheduled) meta.append(' · ', el('span', 'gold', String(scheduled)), ' scheduled');
  if (pending) meta.append(' · ', el('b', '', String(pending)), ' unsaved');

  const g = $('planGrid');
  g.replaceChildren(el('div'));
  for (const day of days) {
    const pairs = day.cells.map((c) => [day.date, c]);
    const n = pairs.filter(([d, c]) => planEligible(c) && planSelected(d, c)).length;
    const h = el('button', 'dh' + (day.date === D.today ? ' today' : ''), day.date === D.today ? 'TODAY' : `${fmt(day.date, { weekday: 'short' }).toUpperCase()} ${dayOf(day.date).getUTCDate()}`);
    h.type = 'button'; h.disabled = locked || !pairs.some(([, c]) => planEligible(c));
    h.setAttribute('aria-label', `Select or clear every meal on ${fmtDay(day.date)}`);
    const count = el('span', n ? '' : 'none', n ? String(n) : '-');
    if (n) count.append(el('i', '', ' TO SELL'));
    h.append(count);
    h.onclick = () => planToggle(pairs);
    g.append(h);
  }
  for (let i = days.length; i < 7; i++) g.append(el('div'));

  const shown = meals().filter((meal) => days.some((d) => d.cells.some((c) => c.meal === meal && c.state !== 'none')));
  for (const meal of shown) {
    const pairs = days.map((d) => [d.date, d.cells.find((c) => c.meal === meal)]).filter(([, c]) => c);
    const mh = el('button', 'mh');
    mh.type = 'button'; mh.disabled = locked || !pairs.some(([, c]) => planEligible(c));
    mh.title = `Select or clear ${meal} all week`;
    mh.setAttribute('aria-label', mh.title);
    mh.append(svgEl(icon(meal)));
    mh.onclick = () => planToggle(pairs);
    g.append(mh);
    for (const day of days) {
      const c = day.cells.find((x) => x.meal === meal);
      if (!c) { g.append(el('div', 'slot out')); continue; }
      if (c.state === 'none') { g.append(el('div', 'slot')); continue; }
      g.append(planCellNode(day.date, c));
    }
    for (let i = days.length; i < 7; i++) g.append(el('div', 'slot out'));
  }
  if (!shown.length) {
    const empty = el('div', 'empty-note', 'Nothing registered these seven days.');
    empty.style.setProperty('grid-column', '1 / -1');
    g.append(empty);
  }
}

function planCellNode(date, c) {
  const key = planKey(date, c), eligible = planEligible(c), on = eligible && planSelected(date, c);
  const n = cellNode(date, planView(date, c));
  const change = planChange(date, c), result = planResults.get(key);
  n.dataset.k = key;
  n.classList.toggle('active', key === planPicked);
  // Every future meal can be sold here, so the ledger's gold call-to-sell border would be everywhere.
  n.classList.remove('sellable');
  if (change) n.classList.add('plan-' + change);
  if (result?.error || (c.schedule_issue && !c.schedule && !on)) n.classList.add('plan-failed');
  if (c.schedule?.retry_at) n.classList.add('plan-retry');
  // Kept meals are exceptions to recurring rules; a stamp tells them apart from plain unsold ones.
  if (eligible && c.kept && !on) { n.classList.add('plan-kept'); n.append(el('span', 'stamp long', 'KEPT')); }
  n.tabIndex = 0;
  n.setAttribute('role', 'button');
  if (eligible) n.setAttribute('aria-pressed', String(on));
  else n.removeAttribute('aria-pressed');
  const what = `${cap(c.meal)}, ${fmtDay(date)}`;
  n.setAttribute('aria-label', c.task ? `${what}: selling now. Open its sale.`
    : !eligible ? `${what}: ${c.state}`
    : on ? `${what}: set to sell${change ? ', unsaved' : ''}. Settings beside the grid.`
    : `${what}: keeping it${c.kept ? ', out of recurring rules' : ''}. Select to sell.`);
  return n;
}

// The settings panel: one meal's settings, or the bulk form when no meal
// is open.
function renderPlanInspector() {
  const panel = $('planInspector');
  const hit = planPicked && planFind(planPicked);
  $('planBulk').hidden = !!hit;
  panel.hidden = !hit;
  $('planSideK').textContent = hit ? 'Meal settings' : 'Selected meals';
  const count = (D.upcoming || []).reduce((n, d) => n + d.cells.filter((c) => planEligible(c) && planSelected(d.date, c)).length, 0);
  $('planBulkCount').textContent = count ? `${count} meal${count === 1 ? '' : 's'} set to sell` : 'No meals set to sell';
  if (!hit) { panel.replaceChildren(); return; }
  const [date, c] = hit;
  panel.replaceChildren();
  const head = el('div', 'plan-head');
  const name = el('div', 'nm', `${messName(c.mess)} ${c.diet ? DIET_LABEL[c.diet].toLowerCase() + ' ' : ''}${c.meal}`);
  name.style.setProperty('color', messColor(c.mess));
  const close = el('button', 'pg', '×'); close.type = 'button'; close.setAttribute('aria-label', 'Close meal settings');
  close.onclick = () => { planPicked = ''; renderPlanner(); };
  head.append(name, close);
  panel.append(head, el('div', 'ln c8', `${fmtDay(date)}, ${c.meal} ${hm(c.start)}-${hm(c.end)}`));
  const result = planResults.get(planPicked);
  if (result) panel.append(el('p', result.error ? 'sell-err' : 'sell-note', result.error || result.message));
  if (c.schedule_issue && !c.schedule) panel.append(el('p', 'sell-err', `${cap(c.schedule_state)}: ${c.schedule_issue}`));
  if (c.kept && !c.schedule) {
    const allow = el('button', 'sell-btn', 'ALLOW RULES AGAIN'); allow.type = 'button'; allow.disabled = planLocked() || c.end <= now();
    allow.onclick = async () => {
      planBusy = true; renderPlanner();
      try { await send('POST', 'api/recurring-rules', { action: 'allow', date, meal: c.meal }); await load(); }
      catch (e) { planResults.set(planPicked, { error: e.message }); }
      finally { planBusy = false; renderPlanner(); }
    };
    panel.append(el('p', 'sell-note', 'Kept out of recurring rules. Allowing it again lets a matching rule schedule it, possibly right away.'), planActs(allow));
  }
  if (c.schedule?.rule_id) panel.append(planRuleNote(c.schedule));
  const rar = (t, cls) => panel.append(el('div', 'rar ' + cls, t));

  if (c.task) {
    if (planDrafts.has(planPicked)) panel.append(el('p', 'sell-err', 'This meal started selling while you were editing. Your other drafts are kept. Use the sale manager for this meal.'));
    panel.append(el('div', 'ln', `Selling now for ${priceText(c.task)}.`));
    const manage = el('button', 'sell-btn gold', 'MANAGE SALE'); manage.type = 'button'; manage.disabled = planBusy; manage.onclick = () => openSell(date, c);
    panel.append(planActs(manage));
    rar('SELLING', 'c6');
    return;
  }
  if (!planEligible(c)) {
    panel.append(el('p', 'sell-note', c.schedule?.detail || { ate: 'You ate this one.', wasted: 'Registered, never sold, never eaten.', sold: 'Already sold.' }[c.state] || `This meal is ${c.state}.`));
    rar(c.state.toUpperCase(), 'c8');
    return;
  }

  const draft = planDraft(date, c);
  if (!draft.selected) {
    panel.append(el('div', 'ln c7', draft.base ? 'Its saved schedule is removed when you save.' : c.kept ? 'Registered. Kept, not set to sell.' : 'Registered. Not set to sell.'));
    if (draft.base && planHasRules()) panel.append(el('p', 'sell-note', 'Recurring rules will skip it too, until you allow it again here.'));
    if (c.ask != null) panel.append(el('div', 'ln c7', `Median ask ${rupees(Math.round(c.ask))}`));
    const sell = el('button', 'sell-btn gold', draft.base ? 'KEEP SCHEDULE' : 'SELL THIS MEAL'); sell.type = 'button'; sell.disabled = planLocked();
    sell.onclick = () => { draft.selected = true; renderPlanner(); };
    panel.append(planActs(sell));
    rar(draft.base ? 'REMOVING' : 'KEEPING', draft.base ? 'cc' : 'c8');
    return;
  }

  if (c.schedule?.retry_at) panel.append(el('p', 'sell-err', `${c.schedule.detail}. Retrying at ${hm(c.schedule.retry_at)}.`));
  if (c.schedule && (draft.base?.id !== c.schedule.id || draft.base?.revision !== c.schedule.revision)) {
    panel.append(el('p', 'sell-err', `This schedule changed. Current saved settings: ${priceText(c.schedule)}, posting ${hm(c.schedule.at)}. Your draft settings are shown below.`));
    const review = el('button', 'sell-btn', 'USE CURRENT SCHEDULE'); review.type = 'button'; review.disabled = planBusy;
    review.onclick = () => { draft.base = { ...c.schedule }; planResults.delete(planPicked); renderPlanner(); };
    panel.append(planActs(review));
  }
  const fieldset = el('fieldset', 'plan-fieldset'); fieldset.disabled = planLocked();
  const slider = priceSlider({ ...c, planner: true, schedule: { min: draft.min, max: draft.max } });
  const fields = el('div', 'plan-fields'); fields.append(...slider.nodes);
  fields.addEventListener('input', () => { [draft.min, draft.max] = slider.get(); renderPlanGrid(); planStatus(); });

  const timing = el('label', 'plan-timing'), before = el('input'), when = el('span', 'plan-when');
  before.type = 'number'; before.min = '-90'; before.max = '30'; before.step = '1'; before.value = Math.round((c.start - draft.at) / 60);
  before.title = 'Minutes before serving starts. Negative posts after it starts, up to 90.';
  const drawWhen = () => { when.replaceChildren('min before · up at ', el('b', '', hm(draft.at))); };
  before.oninput = () => { draft.at = c.start - Number(before.value) * 60; drawWhen(); renderPlanGrid(); planStatus(); };
  drawWhen();
  timing.append(el('span', 'side-k', 'Posts'), before, when);

  const auto = el('button', 'sell-btn plan-autopilot' + (draft.autopilot ? ' on' : ''), draft.autopilot ? '✈ AUTOPILOT ON' : '✈ AUTOPILOT OFF');
  auto.type = 'button'; auto.setAttribute('aria-pressed', String(draft.autopilot));
  auto.onclick = () => { draft.autopilot = !draft.autopilot; renderPlanner(); };
  auto.title = 'Sells without waiting on you; skips buyers who would need your help.';
  fields.append(timing, auto);
  fieldset.append(fields);
  panel.append(fieldset);

  const keep = el('button', 'sell-btn', 'KEEP MEAL'); keep.type = 'button'; keep.disabled = planLocked();
  keep.onclick = () => { draft.selected = false; renderPlanner(); };
  const acts = [keep];
  if (c.schedule) {
    const start = el('button', 'sell-btn gold', 'START NOW'); start.type = 'button'; start.disabled = planLocked();
    start.onclick = async () => {
      if (!confirm(`Start selling ${c.meal} now at the saved prices?`)) return;
      planBusy = true; renderPlanner();
      try { await send('POST', 'api/sell-action', { action: 'start', now: true, date, meal: c.meal, min: c.schedule.min, max: c.schedule.max, autopilot: !!c.schedule.autopilot }); planDrafts.delete(planPicked); await load(); }
      catch (e) { planResults.set(planPicked, { error: e.message }); }
      finally { planBusy = false; renderPlanner(); }
    };
    acts.push(start);
  }
  panel.append(planActs(...acts));
  const change = planChange(date, c);
  if (c.schedule?.retry_at) rar('RETRYING', 'c6');
  else if (change === 'new') rar('TO SCHEDULE · UNSAVED', 'ce');
  else if (change === 'edit') rar('EDITED · UNSAVED', 'ce');
  else rar('SCHEDULED', 'ce');
}

const planHasRules = () => typeof recurringRules !== 'undefined' && recurringRules.some((r) => r.enabled);

// planRuleNote names the rule behind a schedule and says what editing it here does.
function planRuleNote(sc) {
  const r = typeof recurringRules !== 'undefined' ? recurringRules.find((x) => x.id === sc.rule_id) : null;
  const name = r?.name || `Rule #${sc.rule_id}`;
  const rel = r?.pricing === 'median' ? r.relative?.[sc.meal] : null;
  const note = el('p', 'sell-note plan-rule');
  const link = el('a', 'nav-link', 'Edit rule'); link.href = '#recurring-rule=' + sc.rule_id;
  note.append(`From ${name}. `);
  if (sc.overridden) note.append('Edited here, so rule changes leave it alone. ');
  else if (rel) note.append(`Price follows the median at ${signedPct(rel.low)} to ${signedPct(rel.high)}. Changing it here fixes it for this meal. `);
  else note.append('Changing it here keeps it as is when the rule changes. ');
  note.append(link);
  return note;
}

const planActs = (...buttons) => { const a = el('div', 'sell-acts plan-acts'); a.append(...buttons); return a; };

function planApply(days) {
  const min = Number($('planMin').value), max = Number($('planMax').value), before = Number($('planBefore').value);
  if (!Number.isInteger(min) || !Number.isInteger(max) || min < 1 || max < min || max > 10000 || !Number.isInteger(before) || before < -90 || before > 30) {
    $('planError').textContent = 'Use whole rupees (1-10,000, opening at least minimum) and posting time from 30 minutes before to 90 minutes after serving.'; return;
  }
  for (const day of days) for (const c of day.cells) {
    if (!planEligible(c) || !(planDrafts.get(planKey(day.date, c))?.selected ?? !!c.schedule)) continue;
    Object.assign(planDraft(day.date, c), { min, max, at: c.start - before * 60, autopilot: $('planAuto').checked });
  }
  renderPlanner();
}

$('planGrid').addEventListener('click', (e) => {
  const n = e.target.closest('.cell');
  if (n && n.dataset.k) planClick(n.dataset.k);
});
$('planGrid').addEventListener('keydown', (e) => {
  const n = e.target.closest('.cell');
  if (!n || !n.dataset.k || (e.key !== 'Enter' && e.key !== ' ')) return;
  e.preventDefault();
  planClick(n.dataset.k);
  // The grid was rebuilt; keep focus on the same meal.
  for (const x of $('planGrid').querySelectorAll('.cell')) if (x.dataset.k === n.dataset.k) x.focus();
});
$('planPrev').onclick = () => { planPage--; renderPlanner(); };
$('planNext').onclick = () => { planPage++; renderPlanner(); };
$('planWeek').onclick = () => planSelect(planDays(), true);
$('planClearWeek').onclick = () => planSelect(planDays(), false);
$('planAll').onclick = () => planSelect(D.upcoming || [], true);
$('planClear').onclick = () => planSelect(D.upcoming || [], false);
$('planReset').onclick = () => { planDrafts.clear(); planResults.clear(); renderPlanner(); };
$('planApply').onclick = () => planApply(D.upcoming || []);
$('planRangeSelect').onclick = () => {
  const from = $('planFrom').value, to = $('planTo').value, meal = $('planMeal').value;
  if (!from || !to || from > to) { $('planError').textContent = 'Choose a valid date range.'; return; }
  const days = (D.upcoming || []).filter((d) => d.date >= from && d.date <= to && (!$('planWeekdays').checked || ![0, 6].includes(dayOf(d.date).getUTCDay()))).map((d) => ({ ...d, cells: d.cells.filter((c) => !meal || c.meal === meal) }));
  planSelect(days, true);
};
$('planSave').onclick = async () => {
  const actions = planActions();
  for (const { body } of actions) {
    if (body.action === 'plan' && (!Number.isInteger(body.at) || !Number.isInteger(body.min) || !Number.isInteger(body.max) || body.min < 1 || body.max < body.min || body.max > 10000)) {
      $('planError').textContent = 'Use whole rupees: minimum at least ₹1, maximum at least the minimum and at most ₹10,000.';
      return;
    }
  }
  planBusy = true;
  renderPlanner();
  let failures = 0;
  try {
    for (const { key, body } of actions) {
      try {
        const res = await send('POST', 'api/sell-action', body);
        planDrafts.delete(key);
        planResults.set(key, { message: body.action === 'unplan' ? 'Removed' : res?.at ? 'Scheduled' : 'Selling' });
      } catch (e) { failures++; planResults.set(key, { error: e.message }); }
    }
  } finally {
    await load(); if (typeof loadRecurringRules === 'function') await loadRecurringRules(); planBusy = false; renderPlanner();
    $('planError').textContent = failures ? `${failures} meal(s) could not be saved. Open the marked meals for details; Save changes retries only unfinished changes.` : D.sell_off || '';
  }
};

window.addEventListener('beforeunload', (e) => {
  if (D && (planActions().length || [...planResults.values()].some((r) => r.error))) { e.preventDefault(); e.returnValue = ''; }
});
