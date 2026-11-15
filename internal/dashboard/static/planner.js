'use strict';

let planPage = 0;
let planBusy = false;
const planDrafts = new Map();
const planResults = new Map();
const planKey = (date, c) => `${date}|${c.meal}`;
const planEligible = (c) => !c.task && (!c.schedule || !c.schedule.state || c.schedule.state === 'waiting') && (c.schedule || (c.state === 'open' && c.sellable));
const planDays = () => (D.upcoming || []).slice(planPage * 7, planPage * 7 + 7);

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

function planStatus() {
  const actions = planActions(), starts = actions.filter((a) => a.body.action === 'plan' && !a.body.id);
  const immediate = starts.filter((a) => a.body.at <= now()).length;
  const updates = actions.filter((a) => a.body.action === 'plan' && a.body.id).length;
  const drops = actions.filter((a) => a.body.action === 'unplan').length;
  $('planStatus').textContent = planBusy ? 'Saving...' : actions.length ? `${starts.length} new, ${updates} edits, ${drops} removals. ${immediate} start selling immediately.` : 'No unsaved schedules';
  $('planSave').disabled = planBusy || !actions.length || !!D.sell_off;
  $('planReset').disabled = planBusy || !planDrafts.size;
}

function planSelect(days, selected) {
  for (const day of days) for (const c of day.cells) {
    if (planEligible(c)) planDraft(day.date, c).selected = selected;
  }
  renderPlanner();
}

let planPicked = '';

function renderPlanner() {
  $('planner').hidden = !D.owner;
  if (!D.owner) return;
  for (const day of D.upcoming || []) for (const c of day.cells) {
    const key = planKey(day.date, c), draft = planDrafts.get(key);
    if (!draft) continue;
    if (c.schedule && draft.selected && c.schedule.min === draft.min && c.schedule.max === draft.max && c.schedule.at === draft.at && !!c.schedule.autopilot === draft.autopilot) {
      planDrafts.delete(key); planResults.set(key, { message: 'Scheduled' });
    } else if (!planEligible(c) && (draft.selected || draft.base)) {
      planResults.set(key, { error: 'This meal changed or started elsewhere. Review it; your other drafts are kept.' });
    }
  }
  const pickedStillEditable = (D.upcoming || []).some((day) => day.cells.some((c) => planKey(day.date, c) === planPicked && planEligible(c)));
  if (!planBusy && pickedStillEditable && $('planInspector').contains(document.activeElement) && document.activeElement.matches('input')) return;
  const days = planDays();
  $('planSharedFields').disabled = planBusy || !!D.sell_off;
  $('planPrev').disabled = planBusy || planPage === 0;
  $('planNext').disabled = planBusy || (planPage + 1) * 7 >= (D.upcoming || []).length;
  for (const id of ['planWeek', 'planClearWeek', 'planAll', 'planClear', 'planRangeSelect', 'planApply']) $(id).disabled = planBusy || !!D.sell_off;
  $('planFrom').min = $('planTo').min = D.today;
  $('planFrom').max = $('planTo').max = (D.upcoming || []).at(-1)?.date || D.today;
  if (!$('planFrom').value) $('planFrom').value = D.today;
  if (!$('planTo').value) $('planTo').value = days.at(-1)?.date || D.today;
  if (!$('planBefore').value) $('planBefore').value = String(-(D.list_at || 0) / 60);
  $('planRange').textContent = days.length ? `${fmtDay(days[0].date)} - ${fmtDay(days[days.length - 1].date)}` : '';
  const grid = $('planGrid'), side = $('planSide');
  const mobile = !!window.matchMedia?.('(max-width: 760px)')?.matches;
  grid.after(side);
  side.hidden = mobile && !days.some((d) => d.cells.some((c) => planKey(d.date, c) === planPicked));
  grid.replaceChildren();
  const heads = el('div', 'plan-head-row'); heads.append(el('span', 'side-k', 'Day'));
  for (const meal of meals()) {
    const h = el('button', 'plan-meal-head'); h.type = 'button'; h.disabled = planBusy || !!D.sell_off;
    h.append(svgEl(icon(meal)), cap(meal));
    h.setAttribute('aria-label', `Select all ${meal} this week`);
    h.onclick = () => {
      for (const day of days) { const c = day.cells.find((x) => x.meal === meal); if (c && planEligible(c)) planDraft(day.date, c).selected = true; }
      renderPlanner();
    };
    heads.append(h);
  }
  grid.append(heads);
  for (const day of days) {
    const row = el('div', 'plan-day');
    const dateHead = el('button', 'plan-day-head'); dateHead.type = 'button'; dateHead.disabled = planBusy || !!D.sell_off;
    dateHead.textContent = fmtDay(day.date); dateHead.setAttribute('aria-label', `Select meals on ${fmtDay(day.date)}`);
    dateHead.onclick = () => planSelect([day], true); row.append(dateHead);
    for (const meal of meals()) {
      const c = day.cells.find((x) => x.meal === meal);
      if (!c) { row.append(el('div', 'slot out')); continue; }
      const key = planKey(day.date, c), eligible = planEligible(c);
      const draft = planDrafts.get(key) || { selected: !!c.schedule };
      const result = planResults.get(key);
      const wrap = el('div', 'plan-cell');
      const label = el('label', `slot item cell plan-pick${draft.selected ? ' is-scheduled' : ' is-open'}${key === planPicked ? ' active' : ''}${result?.error || c.schedule_issue ? ' plan-failed' : ''}`);
      label.style.setProperty('--m', messColor(c.mess));
      const check = el('input', 'plan-check'); check.type = 'checkbox'; check.checked = !!draft.selected;
      check.disabled = !eligible || planBusy || !!D.sell_off;
      check.setAttribute('aria-label', `Sell ${meal} on ${fmtDay(day.date)}`);
      check.onchange = () => { planDraft(day.date, c).selected = check.checked; planPicked = key; renderPlanner(); };
      label.append(check);
      if (draft.selected) {
        const [min, max] = draft.min != null ? [draft.min, draft.max] : sellDefaults(c, sellBounds(c));
        const price = el('div', 'price range'); price.append(el('span', 'r', '\u20b9'), min === max ? String(min) : `${min}-${max}`);
        label.append(price, el('span', 'sell-tag', hm(draft.at ?? c.schedule.at)));
      } else if (c.state !== 'none') {
        const ghost = el('div', 'ghost'); ghost.append(svgEl(messIcon(c.mess))); label.append(ghost);
      }
      const state = result?.error ? 'Review' : c.task ? 'Selling' : c.schedule ? c.schedule.retry_at ? 'Retrying' : 'Scheduled' : draft.selected ? 'To schedule' : c.schedule_state || ({ open: 'Available', none: 'Not registered', ate: 'Used', wasted: 'Ended' }[c.state] || cap(c.state));
      label.title = `${fmtDay(day.date)} ${cap(meal)} - ${state}${c.schedule_issue ? ': ' + c.schedule_issue : ''}`;
      wrap.append(label);
      const edit = el('button', 'plan-edit', '...'); edit.type = 'button'; edit.disabled = planBusy;
      edit.setAttribute('aria-label', `Settings for ${meal} on ${fmtDay(day.date)}`);
      edit.setAttribute('aria-expanded', String(planPicked === key)); edit.setAttribute('aria-controls', 'planInspector');
      edit.onclick = () => { planPicked = planPicked === key ? '' : key; renderPlanner(); }; wrap.append(edit);
      const caption = el('span', 'plan-caption'); caption.append(el('b', '', cap(meal)), el('span', '', messName(c.mess)), el('span', result?.error ? 'red' : '', state));
      wrap.append(caption); row.append(wrap);
    }
    grid.append(row);
    if (mobile && day.cells.some((c) => planKey(day.date, c) === planPicked)) grid.append(side);
  }
  renderPlanInspector();
  $('planError').textContent = D.sell_off || '';
  planStatus();
}

function renderPlanInspector() {
  const panel = $('planInspector'); panel.replaceChildren();
  const day = planDays().find((d) => d.cells.some((c) => planKey(d.date, c) === planPicked));
  const c = day && day.cells.find((c) => planKey(day.date, c) === planPicked);
  if (!c) { panel.append(el('div', 'nm', 'Pick a meal'), el('p', 'lore', 'Tick a slot to sell it. Open its settings with the three dots.')); return; }
  panel.append(el('div', 'nm', `${messName(c.mess)} ${c.meal}`), el('div', 'plan-date', fmtDay(day.date)));
  const result = planResults.get(planPicked);
  if (result) panel.append(el('p', result.error ? 'sell-err' : 'sell-note', result.error || result.message));
  if (c.schedule_issue) panel.append(el('p', 'sell-err', `${cap(c.schedule_state)}: ${c.schedule_issue}`));
  if (c.task) {
    if (planDrafts.has(planPicked)) panel.append(el('p', 'sell-err', 'This meal started selling while you were editing. Your other drafts are kept. Use the sale manager for this meal.'));
    const manage = el('button', 'sell-btn', 'Manage sale'); manage.type = 'button'; manage.disabled = planBusy; manage.onclick = () => openSell(day.date, c); panel.append(manage); return;
  }
  if (!planEligible(c)) { panel.append(el('p', 'sell-note', c.schedule?.detail || `This meal is ${c.state}.`)); return; }
  const draft = planDraft(day.date, c);
  if (c.schedule?.retry_at) panel.append(el('p', 'sell-err', `${c.schedule.detail}. Retrying at ${hm(c.schedule.retry_at)}.`));
  const slider = priceSlider({ ...c, planner: true, schedule: { min: draft.min, max: draft.max } });
  const fields = el('div', 'plan-fields'); fields.append(...slider.nodes);
  fields.addEventListener('input', () => { [draft.min, draft.max] = slider.get(); planStatus(); });
  const timing = el('label', 'plan-timing', 'Minutes before serving'), before = el('input');
  before.type = 'number'; before.min = '-90'; before.max = '30'; before.step = '1'; before.value = Math.round((c.start - draft.at) / 60);
  before.oninput = () => { draft.at = c.start - Number(before.value) * 60; planStatus(); }; timing.append(before);
  const autoLabel = el('label', 'plan-auto'), auto = el('input'); auto.type = 'checkbox'; auto.checked = draft.autopilot;
  auto.onchange = () => { draft.autopilot = auto.checked; planStatus(); }; autoLabel.append(auto, 'Autopilot');
  fields.append(timing, el('p', 'sell-note', `Serving ${hm(c.start)}-${hm(c.end)}. Negative minutes post after serving starts.`), autoLabel, el('p', 'sell-note', 'Autopilot skips buyers who need your help.'));
  const fieldset = el('fieldset', 'plan-fieldset'); fieldset.disabled = planBusy || !!D.sell_off; fieldset.append(fields); panel.append(fieldset);
  if (c.schedule) {
    const start = el('button', 'sell-btn', 'Start now'); start.type = 'button'; start.disabled = planBusy || !!D.sell_off;
    start.onclick = async () => {
      if (!confirm(`Start selling ${c.meal} now at the saved prices?`)) return;
      planBusy = true; renderPlanner();
      try { await send('POST', 'api/sell-action', { action: 'start', now: true, date: day.date, meal: c.meal, min: c.schedule.min, max: c.schedule.max, autopilot: !!c.schedule.autopilot }); planDrafts.delete(planPicked); await load(); }
      catch (e) { planResults.set(planPicked, { error: e.message }); }
      finally { planBusy = false; renderPlanner(); }
    };
    panel.append(start);
  }
}

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
 const days = (D.upcoming || []).filter((d) => d.date >= from && d.date <= to && (!$('planWeekdays').checked || ![0,6].includes(dayOf(d.date).getUTCDay()))).map((d) => ({...d, cells: d.cells.filter((c) => !meal || c.meal === meal)}));
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
    await load(); planBusy = false; renderPlanner();
    $('planError').textContent = failures ? `${failures} meal(s) could not be saved. Open the marked meals for details; Save changes retries only unfinished changes.` : D.sell_off || '';
  }
};

window.addEventListener('beforeunload', (e) => {
  if (D && (planActions().length || [...planResults.values()].some((r) => r.error))) { e.preventDefault(); e.returnValue = ''; }
});
