'use strict';

// Made-up data in the shapes of api/state, api/sales, api/attention and
// api/recurring-rules. The widget picker draws its previews from it, so a
// widget that shows your data never looks empty there. Nothing in it is a
// real person: names and numbers are invented.
//
// demoData(now) builds everything for the day `now` falls in (Asia/Kolkata).

function demoData(nowSec) {
  const IST = 19800; // +05:30
  const dayStart = (u) => u - ((u + IST) % 86400);
  const date = (u) => new Date((u + IST) * 1000).toISOString().slice(0, 10);
  const at = (day0, h, m) => day0 + h * 3600 + m * 60;
  const t0 = dayStart(nowSec);
  const today = date(nowSec);

  // A seeded random, so the same day always draws the same history.
  let seed = 7;
  const rnd = () => { seed = (seed * 16807) % 2147483647; return (seed - 1) / 2147483646; };
  const pick = (xs) => xs[Math.floor(rnd() * xs.length)];
  const round5 = (v) => Math.round(v / 5) * 5;

  const MESSES = [
    { name: 'kadamba', veg_only: false }, { name: 'yuktahar', veg_only: true },
    { name: 'palash', veg_only: true }, { name: 'bakul', veg_only: false },
  ];
  const MEALS = ['breakfast', 'lunch', 'dinner'];
  const TIMES = { breakfast: [[7, 15], [10, 30]], lunch: [[12, 15], [14, 45]], dinner: [[19, 15], [21, 45]] };
  const win = (day0, meal) => ({ start: at(day0, ...TIMES[meal][0]), end: at(day0, ...TIMES[meal][1]) });
  const BASE = { breakfast: 30, lunch: 55, dinner: 60 };
  const BUYERS = [
    ['b1', 'Ishaan', '+919000000101'], ['b2', 'Meera', '+919000000102'], ['b3', 'Kabir', '+919000000103'],
    ['b4', 'Tara', '+919000000104'], ['b5', '', '+919000000105'], ['b6', 'Rohan', '+919000000106'],
    ['b7', 'Zoya', '+919000000107'], ['b8', 'Dev', '+919000000108'],
  ];
  // Regulars buy more often: earlier buyers in the list weigh more.
  const buyer = () => BUYERS[Math.min(BUYERS.length - 1, Math.floor(rnd() * rnd() * BUYERS.length * 1.4))];

  // ---------- api/sales ----------

  const sales = [];
  const days = [];
  const recent = date(t0 - 27 * 86400);
  for (let i = 27; i >= 0; i--) {
    const day0 = t0 - i * 86400, d = date(day0);
    const cells = MEALS.map((meal) => {
      const w = win(day0, meal);
      const mess = meal === 'breakfast' ? 'yuktahar' : pick(['kadamba', 'bakul', 'kadamba']);
      const diet = mess === 'yuktahar' ? 'veg' : pick(['veg', 'nonveg', 'veg']);
      const ask = round5(BASE[meal] * (diet === 'nonveg' ? 1.6 : 1) * (0.9 + rnd() * 0.3));
      const c = { meal, mess, diet, start: w.start, end: w.end, ask };
      if (i === 0) return c; // today's are set below
      const r = rnd();
      if (r < 0.12) return { meal, state: 'none', start: w.start, end: w.end };
      if (r < 0.62) {
        const [key, name, phone] = buyer();
        const amount = round5(ask * (0.8 + rnd() * 0.35));
        const offer = w.start + Math.floor(rnd() * 1800);
        sales.push({ date: d, meal, mess, diet, amount, state: 'sold', buyer: key, name, phone, end: w.end, armed_at: offer, offer_at: offer, paid_at: offer + 120 + Math.floor(rnd() * 600), ask });
        return { ...c, state: 'sold', sale: sales.length - 1 };
      }
      return { ...c, state: r < 0.9 ? 'ate' : 'wasted' };
    });
    days.push({ date: d, cells });
  }

  // Today: breakfast eaten, lunch selling or waiting, dinner scheduled.
  const tw = Object.fromEntries(MEALS.map((m) => [m, win(t0, m)]));
  const todayCells = days[days.length - 1].cells;
  const lunchOn = nowSec < tw.lunch.end;
  todayCells[0] = { meal: 'breakfast', mess: 'yuktahar', diet: 'veg', ...tw.breakfast, state: 'ate' };
  const lunchTask = { id: 901, min: 50, max: 80, buyers: [], autopilot: false };
  todayCells[1] = { meal: 'lunch', mess: 'kadamba', diet: 'nonveg', ...tw.lunch, ask: 85, state: lunchOn ? 'selling' : 'wasted', task: lunchOn ? lunchTask : undefined };
  const dinnerAt = tw.dinner.start - 1800;
  const dinnerSched = { id: 501, at: dinnerAt, min: 55, max: 80, autopilot: true, state: 'waiting', revision: 1, rule_id: 1 };
  todayCells[2] = { meal: 'dinner', mess: 'bakul', diet: 'veg', ...tw.dinner, ask: 70, state: 'scheduled', schedule: dinnerSched, opens: tw.lunch.end, opens_after: 'lunch' };

  const upcoming = [];
  const scheduled = [{ id: 501, date: today, meal: 'dinner', base_meal: 'dinner', mess: 'bakul', at: dinnerAt, ends: tw.dinner.end, min: 55, max: 80, autopilot: true, opens: tw.lunch.end, opens_after: 'lunch' }];
  for (let i = 1; i <= 27; i++) {
    const day0 = t0 + i * 86400, d = date(day0);
    const cells = MEALS.map((meal, k) => {
      const w = win(day0, meal);
      if ((i + k) % 7 === 3) return { meal, state: 'none', start: w.start, end: w.end };
      const mess = meal === 'breakfast' ? 'yuktahar' : k === 1 ? 'kadamba' : 'bakul';
      const diet = mess === 'yuktahar' ? 'veg' : 'veg';
      const c = { meal, mess, diet, start: w.start, end: w.end, ask: BASE[meal], state: 'open', sellable: i < 2 };
      if (i <= 3 && meal !== 'lunch') {
        const sc = { id: 510 + i * 3 + k, at: w.start - 900, min: round5(BASE[meal] * 0.6), max: round5(BASE[meal] * 1.1), autopilot: true, state: 'waiting', revision: 1, rule_id: 1 };
        scheduled.push({ id: sc.id, date: d, meal, base_meal: meal, at: sc.at, ends: w.end, min: sc.min, max: sc.max, autopilot: true });
        return { ...c, state: 'scheduled', schedule: sc };
      }
      return c;
    });
    upcoming.push({ date: d, cells });
  }
  upcoming.unshift({ date: today, cells: todayCells });

  const market = [];
  for (const meal of MEALS) {
    for (const [diet, k] of [['veg', 1], ['nonveg', 1.6], ['egg', 1.3]]) {
      const m = BASE[meal] * k;
      market.push({ meal, diet, q1: round5(m * 0.8), median: round5(m), q3: round5(m * 1.2), n: 40 });
    }
  }

  const monthStart = today.slice(0, 8) + '01';
  const inMonth = sales.filter((s) => s.date >= monthStart);
  const wasted = days.flatMap((d) => (d.date >= monthStart ? d.cells : [])).filter((c) => c.state === 'wasted');
  const N = {
    now: nowSec, tz: 'Asia/Kolkata', today, from: days[0].date, recent,
    messes: MESSES,
    windows: MEALS.map((meal) => ({ meal, date: today, ...tw[meal] })),
    days, upcoming, sales, market,
    month: {
      start: monthStart, fee: 4800, earned: inMonth.reduce((a, s) => a + s.amount, 0), sold: inMonth.length,
      wasted: wasted.length, potential: wasted.reduce((a, c) => a + (c.ask || 0), 0), unpriced: 0,
    },
    owner: true,
    friends: [
      { date: today, meal: 'dinner', slot: 'dinner#f1', friend: 'Nikhil', mess: 'palash', diet: 'veg', ...tw.dinner, state: 'scheduled', schedule: { id: 601, at: dinnerAt, min: 40, max: 60, state: 'waiting', revision: 1 } },
      { date: date(t0 - 86400), meal: 'lunch', slot: 'lunch#f1', friend: 'Ananya', mess: 'kadamba', diet: 'veg', ...win(t0 - 86400, 'lunch'), state: 'sold', amount: 55, buyer: '+919000000109' },
    ],
    list_at: 0,
    friend_slots: [],
  };

  // ---------- api/state ----------

  const TEXT = {
    sell: ['selling {m} {meal} @{p}', '{m} {meal} available, {p}/-', 'have {m} {meal}, dm', '{m} {meal} for {p}'],
    buy: ['need {m} {meal}', 'anyone selling {m} {meal}?'],
  };
  const listings = [];
  const posts = [];
  const msales = [];
  const names = ['Arjun', 'Sana', 'Vikram', 'Diya', 'Kunal', 'Neha', 'Aditya', 'Pooja', 'Yash', 'Riya', '', 'Harsh'];
  const nextMeal = nowSec < tw.lunch.end ? 'lunch' : 'dinner';
  for (let i = 0; i < 26; i++) {
    const m = MESSES[i % 4].name;
    const veg = MESSES[i % 4].veg_only;
    const kind = i % 9 === 4 ? 'buy' : i % 11 === 7 ? 'exchange' : 'sell';
    const diet = veg ? 'veg' : pick(['veg', 'nonveg', 'nonveg', '']);
    const price = kind === 'exchange' ? null : rnd() < 0.15 ? null : round5(BASE[nextMeal] * (diet === 'nonveg' ? 1.6 : 1) * (0.75 + rnd() * 0.5));
    const t = nowSec - Math.floor(rnd() * 7000);
    const p = price == null ? 'ask' : String(price);
    const text = pick(TEXT[kind === 'buy' ? 'buy' : 'sell']).replace('{m}', m).replace('{meal}', nextMeal).replace('{p}', p);
    const sold = rnd() < 0.2;
    listings.push({
      id: 1000 + i, kind, mess: m, meal: nextMeal, meal_date: today, diet, price, quantity: i % 13 === 5 ? 2 : 1,
      wants: kind === 'exchange' ? [MESSES[(i + 1) % 4].name] : undefined,
      text, status: sold ? 'sold' : 'available', other_ticks: i % 10 === 3 ? 1 : 0,
      seller: { name: names[i % names.length], phone: `+91900000${String(200 + i).padStart(4, '0')}`, saved: false },
      groups: ['Mess Buy/Sell @ IIIT'], meal_ends_at: tw[nextMeal].end, first_seen_at: t, last_seen_at: t, meal_source: 'text',
    });
    posts.push({ t, id: 1000 + i, mess: m, diet, kind, price: price == null ? undefined : price });
    if (sold) msales.push({ t: t + 300, first: t, id: 1000 + i, mess: m, diet, price: price == null ? undefined : price });
  }

  const lunchSale = {
    id: 901, date: today, meal: 'lunch', base_meal: 'lunch', mess: 'kadamba', diet: 'nonveg', state: 'active', autopilot: false, handoff: '',
    min: 50, max: 80, target: 70, lowest: 65, negotiates: true, started: tw.lunch.start - 600, ends: tw.lunch.end,
    posts: [{ group: 'Mess Buy/Sell @ IIIT', jid: 'g1', seq: 1, state: 'posted', at: tw.lunch.start - 600, after: 2 },
      { group: 'Kadamba Mess Group', jid: 'g2', seq: 1, state: 'posted', at: tw.lunch.start - 590, after: 0 }],
    buyers: [
      { key: 'b2', name: 'Meera', phone: '+919000000102', saved: true, state: 'negotiating', joined: nowSec - 400, ask: 70, offer: 60, bot_msgs: 3, events: [] },
      { key: 'b6', name: 'Rohan', phone: '+919000000106', saved: false, state: 'queued', joined: nowSec - 90, ask: 75, bot_msgs: 1, events: [] },
    ],
    history: [], listings: [1001],
  };
  const D = {
    now: nowSec, tz: 'Asia/Kolkata', messes: MESSES,
    today: MEALS.map((meal) => ({ meal, date: today, ...tw[meal] })),
    listings,
    market: { since: nowSec - 7200, posts, sales: msales },
    mine: [],
    sales: lunchOn ? [lunchSale] : [],
    scheduled,
    nudge: true,
    meals: MEALS,
  };
  const cur = MEALS.find((m) => tw[m].start <= nowSec && nowSec < tw[m].end);
  if (cur) D.current = { meal: cur, date: today, ...tw[cur] };
  else {
    const nx = MEALS.find((m) => tw[m].start > nowSec);
    D.next = nx ? { meal: nx, date: today, ...tw[nx] } : { meal: 'breakfast', date: date(t0 + 86400), ...win(t0 + 86400, 'breakfast') };
  }

  const attention = [
    { id: 'a1', kind: 'refund', title: 'Refund Kabir ₹60', detail: 'They paid twice for the same lunch.', date: date(t0 - 86400), meal: 'lunch', task: 880, resolvable: true },
    { id: 'a2', kind: 'rule', title: 'Weekday dinners skipped a meal', detail: 'Friday dinner is not registered.', rule: 1, resolvable: true },
  ];
  const rules = [
    { id: 1, revision: 3, name: 'Weekday dinners', from: date(t0 - 10 * 86400), through: date(t0 + 40 * 86400), weekdays: [1, 2, 3, 4, 5], meals: ['dinner'], min: 55, max: 80, before: 30, autopilot: true, enabled: true, waiting: 6 },
    { id: 2, revision: 1, name: 'Weekend breakfasts', from: date(t0 - 3 * 86400), through: date(t0 + 60 * 86400), weekdays: [0, 6], meals: ['breakfast'], min: 20, max: 35, before: 15, autopilot: false, enabled: true, waiting: 4 },
    { id: 3, revision: 2, name: 'Exam week lunches', from: date(t0 + 20 * 86400), through: date(t0 + 26 * 86400), weekdays: [1, 2, 3, 4, 5], meals: ['lunch'], pricing: 'median', relative: { lunch: { low: -15, high: 20 } }, min: 40, max: 80, before: 0, autopilot: false, enabled: false, waiting: 0 },
  ];
  return { state: D, sales: N, attention, rules };
}

if (typeof module !== 'undefined') module.exports = { demoData };
