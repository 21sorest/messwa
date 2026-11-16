'use strict';

// Recurring rules: a list in its own chest, edited in a sheet like the
// sell and friend forms. The sheet's fields are static HTML so a list
// refresh never wipes a half-edited rule.

let recurringRules = [];
let recurringBusy = false;
let recurringPreviewKey = '';
let recurringInitialized = false;
let recurringLoadedAt = 0;
let recurringLoading = false;
let recurringSlider = null;
let recurringEditing = null;
let recurringPriceLabel = null;
// The % of median a meal sells at, per meal; kept while meals are toggled.
let recurringRelative = {};
const recurringRelDefault = { low: -15, high: 25 };
const recurringDays = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
// Monday first, as the ledger reads.
const recurringDayOrder = [1, 2, 3, 4, 5, 6, 0];
const recurringOffsets = [30, 20, 15, 10, 5, 0, -10, -15, -30, -45, -60, -90];

function ruleForm() {
  return {
    id: Number($('ruleID').value || 0), revision: Number($('ruleRevision').value || 0),
    name: $('ruleName').value.trim(), from: $('ruleFrom').value, through: $('ruleThrough').value,
    weekdays: [...$('ruleDays').querySelectorAll('input:checked')].map((i) => Number(i.value)).sort((a, b) => a - b),
    meals: [...$('ruleMeals').querySelectorAll('input:checked')].map((i) => i.value),
    min: Number($('ruleMin').value), max: Number($('ruleMax').value), before: Number($('ruleBefore').value),
    autopilot: $('ruleAuto').checked, enabled: $('ruleEnabled').checked,
    ...rulePricingForm(),
  };
}

function rulePricingMode() {
  return $('rulePricing').querySelectorAll('input:checked')[0]?.value || '';
}

function rulePricingForm() {
  if (rulePricingMode() !== 'median') return { pricing: '' };
  const relative = {};
  for (const i of $('ruleMeals').querySelectorAll('input:checked')) relative[i.value] = { ...(recurringRelative[i.value] || recurringRelDefault) };
  return { pricing: 'median', relative };
}

const signedPct = (v) => `${v > 0 ? '+' : v < 0 ? '−' : '±'}${Math.abs(v)}%`;

// ruleMedian is the recent median ask for meal in the diet the owner
// usually registers, for the sheet's estimate. The service prices each
// meal from its own mess, diet and weekday, so this is a guide.
function ruleMedian(meal) {
  if (typeof marketFor !== 'function' || !D?.market) return null;
  const cell = (D.upcoming || []).flatMap((d) => d.cells).find((c) => c.meal === meal && c.diet);
  const m = marketFor(meal, cell?.diet || 'veg');
  return m ? m.median : null;
}

function renderRuleRelative() {
  const median = rulePricingMode() === 'median';
  const box = $('ruleRelative');
  box.hidden = !median;
  if (recurringPriceLabel) recurringPriceLabel.textContent = median ? 'If a meal has no median yet, use' : 'Price';
  box.replaceChildren();
  if (!median) return;
  const r5 = (v) => Math.max(1, Math.round(v / 5) * 5);
  for (const input of $('ruleMeals').querySelectorAll('input:checked')) {
    const meal = input.value;
    const rel = recurringRelative[meal] || (recurringRelative[meal] = { ...recurringRelDefault });
    const row = el('div', 'rule-rel-row'), name = el('span', 'rr-meal'), est = el('span', 'rr-est');
    name.append(svgEl(icon(meal)), cap(meal));
    const field = (label, key) => {
      const l = el('label', 'rr-field'), wrap = el('span', 'rr-in'), n = el('input');
      Object.assign(n, { type: 'number', min: -90, max: key === 'low' ? 200 : 300, step: 5, value: rel[key] });
      n.setAttribute('aria-label', `${cap(meal)} ${label.toLowerCase()}, percent above or below the median`);
      n.addEventListener('input', () => { rel[key] = Number(n.value); draw(); });
      wrap.append(n, el('i', '', '%'));
      l.append(el('span', '', label), wrap);
      return l;
    };
    const draw = () => {
      const m = ruleMedian(meal);
      est.replaceChildren();
      if (m == null) { est.append(el('b', '', `${signedPct(rel.low)} to ${signedPct(rel.high)}`), el('small', '', 'no median yet')); return; }
      const lo = r5(m * (100 + rel.low) / 100), hi = Math.max(lo, r5(m * (100 + rel.high) / 100));
      est.append(el('b', '', lo === hi ? `≈ ₹${lo}` : `≈ ₹${lo}-${hi}`), el('small', '', `median ₹${Math.round(m)}`));
    };
    row.append(name, field('Lowest', 'low'), field('Opens at', 'high'), est);
    draw();
    box.append(row);
  }
  if (!box.children.length) box.append(el('p', 'sell-note', 'Pick a meal to set its price.'));
  else box.append(el('p', 'sell-note', 'Each meal goes by its own recent median for your mess and diet, on the same weekday. Waiting meals follow the market until they go up; meals you edit by hand keep your prices.'));
}

function rulePayload(action) {
  return { action, rule: ruleForm(), apply_waiting: $('ruleApply').checked };
}

function invalidateRulePreview() {
  recurringPreviewKey = '';
  $('ruleSave').disabled = true;
  $('rulePreviewSum').hidden = true;
  $('rulePreviewText').textContent = 'Preview the rule to see which meals it schedules, then save.';
}

function setRuleBusy(busy) {
  recurringBusy = busy;
  $('ruleFields').disabled = busy;
  $('ruleNew').disabled = busy;
  for (const id of ['rulePreview', 'ruleCancel', 'ruleDeleteAsk', 'ruleDelete', 'ruleDeleteBack']) $(id).disabled = busy;
  if (busy) $('ruleSave').disabled = true;
  for (const b of $('ruleList').querySelectorAll('button')) b.disabled = busy;
}

// "Mon-Fri", "Weekends", "Every day" or "Mon, Wed, Fri".
function ruleDaysText(days) {
  const set = new Set(days);
  if (set.size === 7) return 'Every day';
  if (set.size === 5 && [1, 2, 3, 4, 5].every((d) => set.has(d))) return 'Mon-Fri';
  if (set.size === 2 && set.has(0) && set.has(6)) return 'Weekends';
  return recurringDayOrder.filter((d) => set.has(d)).map((d) => recurringDays[d]).join(', ');
}

function ruleTimeText(before) {
  if (before > 0) return `${before} min before serving`;
  if (before < 0) return `${-before} min into serving`;
  return 'when serving starts';
}

function ruleDateText(date) {
  return typeof fmtDay === 'function' ? fmtDay(date) : date;
}

function ruleChip(parent, type, value, label, checked, art) {
  const l = el('label', 'chip'), input = el('input'), span = el('span');
  input.type = type; input.value = String(value); input.checked = checked;
  if (art) span.append(svgEl(art));
  span.append(label);
  l.append(input, span);
  parent.append(l);
}

function fillRuleTimes(before) {
  const select = $('ruleBefore');
  if (typeof Option === 'undefined') { select.value = String(before); return; }
  const offsets = recurringOffsets.includes(before) ? recurringOffsets : [...recurringOffsets, before].sort((a, b) => b - a);
  select.replaceChildren(...offsets.map((m) => new Option(ruleTimeText(m).replace(/^./, (c) => c.toUpperCase()), String(m))));
  select.value = String(before);
}

function drawRulePrice(min, max) {
  $('ruleMin').value = min; $('ruleMax').value = max;
  if (typeof priceSlider !== 'function') return;
  recurringSlider = priceSlider({ planner: true, schedule: { min, max } });
  const box = $('rulePrice');
  recurringPriceLabel = el('span', 'fr-k', 'Price');
  box.replaceChildren(recurringPriceLabel, ...recurringSlider.nodes);
  // Runs before the form's listener, so the preview check sees new prices.
  box.oninput = () => { const [lo, hi] = recurringSlider.get(); $('ruleMin').value = lo; $('ruleMax').value = hi; };
}

function showRuleSheet(open) {
  $('ruleModal').hidden = !open;
  if (open) setTimeout(() => $('ruleName').focus?.(), 0);
}

function closeRuleSheet() {
  if (recurringBusy) return;
  showRuleSheet(false);
}

function showRuleDelete(on) {
  $('ruleDeleteBox').hidden = !on;
  $('ruleActs').hidden = on;
  $('rulePreviewBox').hidden = on;
}

function editRecurringRule(rule, open = true) {
  if (rule || D) recurringInitialized = true;
  const r = rule || {
    id: 0, revision: 0, name: '', from: D?.today || '', through: D ? addDays(D.today, 27) : '',
    weekdays: [1, 2, 3, 4, 5], meals: ['lunch'], min: 40, max: 80,
    before: D ? -D.list_at / 60 : 15, autopilot: false, enabled: true, waiting: 0,
  };
  recurringEditing = r.id ? r : null;
  for (const [id, key] of [['ruleID', 'id'], ['ruleRevision', 'revision'], ['ruleName', 'name'], ['ruleFrom', 'from'], ['ruleThrough', 'through']]) $(id).value = r[key];
  $('ruleAuto').checked = !!r.autopilot; $('ruleEnabled').checked = !!r.enabled;
  $('ruleApply').checked = false; $('ruleRemoveWaiting').checked = false;
  $('ruleApplyRow').hidden = !r.id || !r.waiting;
  $('ruleWaitingCount').textContent = String(r.waiting || 0);
  const days = $('ruleDays'), mealBox = $('ruleMeals');
  days.replaceChildren(el('legend', '', 'Days')); mealBox.replaceChildren(el('legend', '', 'Meals'));
  recurringDayOrder.forEach((d) => ruleChip(days, 'checkbox', d, recurringDays[d], r.weekdays.includes(d)));
  (D ? meals() : ['breakfast', 'lunch', 'snacks', 'dinner']).forEach((meal) => ruleChip(mealBox, 'checkbox', meal, cap(meal), r.meals.includes(meal), icon(meal)));
  fillRuleTimes(r.before);
  recurringRelative = JSON.parse(JSON.stringify(r.relative || {}));
  const pricing = $('rulePricing');
  pricing.replaceChildren(el('legend', '', 'Price'));
  for (const [value, label] of [['', 'Fixed ₹'], ['median', '% of median']]) {
    ruleChip(pricing, 'radio', value, label, (r.pricing || '') === value);
    const input = pricing.children[pricing.children.length - 1].children[0];
    input.name = 'rulePricing';
  }
  drawRulePrice(r.min, r.max);
  renderRuleRelative();
  $('ruleTitle').textContent = r.id ? r.name : 'New rule';
  $('ruleSub').textContent = r.id
    ? `${r.enabled ? 'Active' : 'Paused'} · ${r.waiting || 0} meal${r.waiting === 1 ? '' : 's'} waiting. Changes apply to meals it schedules from now on.`
    : 'Schedules your registered meals that match, up to four weeks ahead. It checks for new registrations every five minutes.';
  $('ruleDeleteAsk').hidden = !r.id;
  $('ruleError').textContent = r.last_error ? `Last run: ${r.last_error}` : '';
  showRuleDelete(false);
  invalidateRulePreview();
  if (open) showRuleSheet(true);
}

// ruleListPrice is a rule's price for its row: the fixed range, or the
// percentages around the median when every meal shares them.
function ruleListPrice(r) {
  if (r.pricing !== 'median') return r.min === r.max ? `₹${r.min}` : `₹${r.min}-${r.max}`;
  const rels = r.meals.map((m) => r.relative?.[m]).filter(Boolean);
  const same = rels.length && rels.every((x) => x.low === rels[0].low && x.high === rels[0].high);
  return same ? `${signedPct(rels[0].low)}/${signedPct(rels[0].high)} median` : 'vs median';
}

function rulePill(r) {
  if (r.enabled && r.last_error) return ['CHECK', 'cc'];
  return r.enabled ? ['ACTIVE', 'ca'] : ['PAUSED', 'c8'];
}

function renderRecurringRules() {
  const list = $('ruleList'); list.replaceChildren();
  const on = recurringRules.filter((r) => r.enabled).length, waiting = recurringRules.reduce((n, r) => n + (r.waiting || 0), 0);
  $('ruleMeta').replaceChildren(el('b', '', String(on)), ' active · ', el('b', '', String(waiting)), ' meals waiting');
  if (!recurringRules.length) {
    list.append(el('div', 'empty-note', 'Selling the same meals every week? Make a rule once. It schedules matching registered meals four weeks ahead, at your prices. Unschedule a meal in the ledger to keep it.'));
    return;
  }
  for (const r of recurringRules) {
    const row = el('button', 'friend live rule-row' + (r.enabled ? '' : ' off'));
    row.type = 'button'; row.disabled = recurringBusy;
    row.setAttribute('aria-label', `Edit recurring rule ${r.name}`);
    row.onclick = () => editRecurringRule(r);
    const ic = el('span', 'fr-ic');
    ic.append(svgEl(r.meals.length === 1 ? icon(r.meals[0]) : mealsAllIcon(r.meals)));
    const what = el('div', 'fr-what');
    what.append(el('b', '', r.name),
      el('span', '', `${ruleDaysText(r.weekdays)} · ${r.meals.map(cap).join(', ')} · up ${ruleTimeText(r.before)}`),
      el('span', '', `${ruleDateText(r.from)} - ${ruleDateText(r.through)}${r.autopilot ? ' · autopilot' : ''}`));
    if (r.enabled && r.last_error) what.append(el('span', 'rule-err', r.last_error));
    const [word, cls] = rulePill(r);
    const right = el('div', 'fr-right');
    right.append(el('span', 'pill ' + cls, word), el('span', 'fr-amt', ruleListPrice(r)), el('span', 'fr-buyer', `${r.waiting} waiting`));
    row.append(ic, what, right);
    list.append(row);
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
    $('ruleListError').textContent = '';
    renderRecurringRules();
    if (!recurringInitialized && D) editRecurringRule(undefined, false);
    openRecurringHash();
  } catch (e) { $('ruleListError').textContent = e.message; }
  finally { recurringLoading = false; }
}

function refreshRecurringRulesView() {
  $('ruleSec').hidden = !D?.owner;
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
  $('ruleSec').scrollIntoView({ block: 'start' });
  editRecurringRule(rule);
  history.replaceState(null, '', location.pathname + location.search);
}

function previewTiles(out, applying) {
  const tiles = [['NEW', out.new, out.new ? 'gold' : ''], ['START NOW', out.starts_now, out.starts_now ? 'gold' : ''], ['KEPT', out.kept, '']];
  if (recurringEditing) tiles.push([applying ? 'UPDATED' : 'UNCHANGED', out.waiting, ''], ['YOUR EDITS', out.preserved_overrides, '']);
  if (out.existing) tiles.push(['ALREADY SET', out.existing, '']);
  if (out.new && out.highest) tiles.push(['PRICES', out.lowest === out.highest ? `₹${out.lowest}` : `₹${out.lowest}-${out.highest}`, 'gold']);
  if (out.fallback) tiles.push(['NO MEDIAN', out.fallback, '']);
  return tiles.map(([k, v, cls]) => { const t = el('div', 'st'); t.append(el('span', '', k), el('b', cls, String(v))); return t; });
}

$('ruleIcon').append?.(svgEl(icon('clock')));
$('ruleForm').addEventListener('input', invalidateRulePreview);
$('ruleForm').addEventListener('change', (e) => {
  invalidateRulePreview();
  if (e.target?.name === 'rulePricing' || e.target?.closest?.('#ruleMeals')) renderRuleRelative();
});
$('ruleNew').onclick = () => editRecurringRule();
$('ruleCancel').onclick = closeRuleSheet;
$('ruleDeleteAsk').onclick = () => showRuleDelete(true);
$('ruleDeleteBack').onclick = () => showRuleDelete(false);
$('ruleModal').addEventListener('click', (e) => { if (e.target === $('ruleModal')) closeRuleSheet(); });
addEventListener('keydown', (e) => { if (e.key === 'Escape' && !$('ruleModal').hidden) { e.stopImmediatePropagation(); closeRuleSheet(); } }, true);

$('rulePreview').onclick = async () => {
  setRuleBusy(true); $('ruleError').textContent = '';
  const payload = rulePayload('preview');
  let ok = false;
  try {
    const out = await send('POST', 'api/recurring-rules', payload);
    recurringPreviewKey = JSON.stringify(rulePayload('save'));
    $('rulePreviewText').textContent = (out.starts_now
      ? `${out.new} new meal${out.new === 1 ? '' : 's'} to schedule; ${out.starts_now} start selling immediately when you save.`
      : `${out.new} new meal${out.new === 1 ? '' : 's'} to schedule. Nothing starts selling right away.`)
      + (out.fallback ? ` ${out.fallback} ${out.fallback === 1 ? 'has' : 'have'} no market median yet and use${out.fallback === 1 ? 's' : ''} the fixed price.` : '');
    $('rulePreviewSum').replaceChildren(...previewTiles(out, payload.apply_waiting));
    $('rulePreviewSum').hidden = false;
    ok = true;
  } catch (e) { $('ruleError').textContent = e.message; invalidateRulePreview(); }
  finally { setRuleBusy(false); $('ruleSave').disabled = !ok; }
};

$('ruleForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = rulePayload('save');
  if (!recurringPreviewKey || recurringPreviewKey !== JSON.stringify(payload)) { invalidateRulePreview(); return; }
  setRuleBusy(true); $('ruleError').textContent = '';
  try {
    const saved = await send('POST', 'api/recurring-rules', payload);
    setRuleBusy(false);
    showRuleSheet(false);
    editRecurringRule(saved, false);
    if (typeof toast === 'function') toast(saved.last_error ? `Rule saved. Scheduling will retry: ${saved.last_error}` : `Saved ${saved.name}. Its meals show in the ledger.`);
    await loadRecurringRules(); await load();
  } catch (error) { $('ruleError').textContent = error.message; }
  finally { setRuleBusy(false); }
});

$('ruleDelete').onclick = async () => {
  const rule = ruleForm(); if (!rule.id) return;
  setRuleBusy(true); $('ruleError').textContent = '';
  try {
    await send('POST', 'api/recurring-rules', { action: 'delete', rule, cancel_waiting: $('ruleRemoveWaiting').checked });
    setRuleBusy(false);
    showRuleSheet(false);
    editRecurringRule(undefined, false);
    if (typeof toast === 'function') toast('Rule deleted. Meals already selling carry on.');
    await loadRecurringRules(); await load();
  } catch (e) { $('ruleError').textContent = e.message; }
  finally { setRuleBusy(false); }
};

addEventListener('hashchange', openRecurringHash);
loadRecurringRules();

function recurringRuleName(id) { return recurringRules.find((r) => r.id === id)?.name || `Rule #${id}`; }
