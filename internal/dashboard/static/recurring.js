'use strict';

let recurringRules = [];
let recurringBusy = false;
let recurringPreviewKey = '';
let recurringInitialized = false;
let recurringLoadedAt = 0;
let recurringLoading = false;
const recurringDays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

function ruleForm() {
  return {
    id: Number($('ruleID').value || 0), revision: Number($('ruleRevision').value || 0),
    name: $('ruleName').value.trim(), from: $('ruleFrom').value, through: $('ruleThrough').value,
    weekdays: [...$('ruleDays').querySelectorAll('input:checked')].map((i) => Number(i.value)),
    meals: [...$('ruleMeals').querySelectorAll('input:checked')].map((i) => i.value),
    min: Number($('ruleMin').value), max: Number($('ruleMax').value), before: Number($('ruleBefore').value),
    autopilot: $('ruleAuto').checked, enabled: $('ruleEnabled').checked,
  };
}

function rulePayload(action) {
  return { action, rule: ruleForm(), apply_waiting: $('ruleApply').checked };
}

function invalidateRulePreview() {
  recurringPreviewKey = '';
  $('ruleSave').disabled = true;
  $('rulePreviewText').textContent = 'Preview to see new schedules, immediate starts and existing meals affected.';
}

function setRuleBusy(busy) {
  recurringBusy = busy;
  $('ruleFields').disabled = busy;
  $('ruleNew').disabled = busy;
  for (const b of $('ruleList').querySelectorAll('button')) b.disabled = busy;
}

function editRecurringRule(rule, open = true) {
  $('ruleEditor').open = open;
  if (rule || D) recurringInitialized = true;
  const r = rule || {
    id: 0, revision: 0, name: '', from: D?.today || '', through: D ? addDays(D.today, 27) : '',
    weekdays: [1, 2, 3, 4, 5], meals: ['lunch'], min: 40, max: 80,
    before: D ? -D.list_at / 60 : 15, autopilot: false, enabled: true,
  };
  for (const [id, key] of [['ruleID', 'id'], ['ruleRevision', 'revision'], ['ruleName', 'name'], ['ruleFrom', 'from'], ['ruleThrough', 'through'], ['ruleMin', 'min'], ['ruleMax', 'max'], ['ruleBefore', 'before']]) $(id).value = r[key];
  $('ruleAuto').checked = !!r.autopilot; $('ruleEnabled').checked = !!r.enabled;
  $('ruleApply').checked = false; $('ruleRemoveWaiting').checked = false;
  $('ruleDays').replaceChildren(); $('ruleMeals').replaceChildren();
  const chip = (parent, value, name, checked) => {
    const label = el('label', 'chip'), input = el('input'); input.type = 'checkbox'; input.value = String(value); input.checked = checked;
    label.append(input, el('span', '', name)); parent.append(label);
  };
  recurringDays.forEach((name, i) => chip($('ruleDays'), i, name, r.weekdays.includes(i)));
  (D ? meals() : ['breakfast', 'lunch', 'snacks', 'dinner']).forEach((meal) => chip($('ruleMeals'), meal, cap(meal), r.meals.includes(meal)));
  $('ruleDelete').hidden = !r.id;
  $('ruleError').textContent = '';
  invalidateRulePreview();
}

function renderRecurringRules() {
  const list = $('ruleList'); list.replaceChildren();
  if (!recurringRules.length) list.append(el('p', 'sell-note', 'No recurring rules yet. Choose meals and weekdays once, and generate schedules as registrations become available.'));
  for (const r of recurringRules) {
    const row = el('div', 'friend'), copy = el('div', 'fr-what'), edit = el('button', 'sell-btn', 'Edit');
    copy.append(el('b', '', r.name), el('span', '', `${r.weekdays.map((d) => recurringDays[d]).join(', ')} · ${r.meals.join(', ')}`), el('span', '', `${r.from} to ${r.through} · ${r.enabled ? 'Enabled' : 'Disabled'} · ${r.waiting} waiting`));
    if (r.last_error) copy.append(el('p', 'sell-err', r.last_error));
    const ic = el('span', 'fr-ic'); ic.append(svgEl(icon('coin')));
    edit.type = 'button'; edit.disabled = recurringBusy; edit.onclick = () => { editRecurringRule(r); $('ruleForm').scrollIntoView({ block: 'nearest' }); };
    row.append(ic, copy, edit); list.append(row);
  }
}

async function loadRecurringRules() {
  if (recurringLoading) return;
  recurringLoading = true;
  try {
    const response = await fetch('api/recurring-rules', { cache: 'no-store' });
    if (response.status === 401 || response.status === 403) return;
    if (!response.ok) throw new Error('Could not refresh recurring rules.');
    recurringRules = await response.json();
    recurringLoadedAt = Date.now();
    renderRecurringRules();
    if (!recurringInitialized && D) editRecurringRule(undefined, false);
    openRecurringHash();
  } catch (e) { $('ruleError').textContent = e.message; }
  finally { recurringLoading = false; }
}

function refreshRecurringRulesView() {
  if (!D?.owner) return;
  if (!recurringInitialized) editRecurringRule(undefined, false);
  if (Date.now() - recurringLoadedAt > 30000) loadRecurringRules();
}

function openRecurringHash() {
  if (recurringBusy) return;
  const match = /^#recurring-rule=(\d+)$/.exec(location.hash);
  if (!match) return;
  const rule = recurringRules.find((r) => r.id === Number(match[1]));
  if (!rule) return;
  $('recurringSection').open = true; editRecurringRule(rule);
  $('recurringSection').scrollIntoView({ block: 'start' });
  history.replaceState(null, '', location.pathname + location.search);
}

$('ruleForm').addEventListener('input', invalidateRulePreview);
$('ruleNew').onclick = () => editRecurringRule();
$('rulePreview').onclick = async () => {
  setRuleBusy(true); $('ruleError').textContent = '';
  const payload = rulePayload('preview');
  try {
    const out = await send('POST', 'api/recurring-rules', payload);
    recurringPreviewKey = JSON.stringify(rulePayload('save'));
    $('rulePreviewText').textContent = `${out.new} new schedules; ${out.starts_now} start selling immediately. ${out.waiting} existing waiting meals${payload.apply_waiting ? ' will be updated or removed' : ' stay unchanged'}. ${out.preserved_overrides} individual overrides preserved; ${out.kept} kept meals skipped; ${out.existing} already handled.`;
    $('ruleSave').disabled = false;
  } catch (e) { $('ruleError').textContent = e.message; invalidateRulePreview(); }
  finally { setRuleBusy(false); }
};

$('ruleForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = rulePayload('save');
  if (!recurringPreviewKey || recurringPreviewKey !== JSON.stringify(payload)) { invalidateRulePreview(); return; }
  setRuleBusy(true); $('ruleError').textContent = '';
  try {
    const saved = await send('POST', 'api/recurring-rules', payload);
    editRecurringRule(saved); $('rulePreviewText').textContent = saved.last_error ? `Rule saved. Generation will retry: ${saved.last_error}` : 'Rule saved. Scheduled meals appear in the planner.';
    await loadRecurringRules(); await load();
  } catch (error) { $('ruleError').textContent = error.message; }
  finally { setRuleBusy(false); }
});

$('ruleDelete').onclick = async () => {
  const rule = ruleForm(); if (!rule.id) return;
  setRuleBusy(true); $('ruleError').textContent = '';
  try {
    await send('POST', 'api/recurring-rules', { action: 'delete', rule, cancel_waiting: $('ruleRemoveWaiting').checked });
    editRecurringRule(); await loadRecurringRules(); await load(); $('rulePreviewText').textContent = 'Rule removed. Active sales continue through their sale manager.';
  } catch (e) { $('ruleError').textContent = e.message; }
  finally { setRuleBusy(false); }
};

addEventListener('hashchange', openRecurringHash);
loadRecurringRules();

function recurringRuleName(id) { return recurringRules.find((r) => r.id === id)?.name || `Rule #${id}`; }
