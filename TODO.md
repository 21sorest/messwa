# Scheduler follow-up

The first scheduler release implements the weekly planner and resilient execution from the scheduler plan. Recurrence remains the next phase, as staged in that plan.

- Add bounded recurring rules with start date, end date, weekdays and meal choices.
- Generate ordinary schedules only for confirmed registrations, using saved rule prices and posting preferences.
- Preserve per-meal exceptions such as keeping one Wednesday lunch; regeneration must not recreate an excepted schedule.
- When deleting a rule, let the user remove its future waiting schedules. Active sales continue through the existing sale manager.
- Test restart regeneration, duplicate prevention, exceptions, rule edits and account isolation before enabling recurring execution.
