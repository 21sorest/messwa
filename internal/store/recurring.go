package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const recurringSchema = `
CREATE TABLE IF NOT EXISTS recurring_rules (
 id INTEGER PRIMARY KEY AUTOINCREMENT,user_id INTEGER NOT NULL,config TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1,enabled INTEGER NOT NULL,deleted INTEGER NOT NULL DEFAULT 0,
 last_error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS planner_kept (
 user_id INTEGER NOT NULL,date TEXT NOT NULL,meal TEXT NOT NULL,PRIMARY KEY(user_id,date,meal)
);`

type RecurringRule struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"-"`
	Revision  int64    `json:"revision"`
	Name      string   `json:"name"`
	From      string   `json:"from"`
	Through   string   `json:"through"`
	Weekdays  []int    `json:"weekdays"` // Sunday=0, in the mess timezone
	Meals     []string `json:"meals"`
	Min       int      `json:"min"`
	Max       int      `json:"max"`
	Before    int      `json:"before"` // minutes before serving; negative means after
	Autopilot bool     `json:"autopilot"`
	Enabled   bool     `json:"enabled"`
	LastError string   `json:"last_error,omitempty"`
	Waiting   int      `json:"waiting"`
}

func (s *Store) RecurringRules(ctx context.Context) ([]RecurringRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,config,revision,enabled,last_error,(SELECT count(*) FROM sell_schedules WHERE rule_id=recurring_rules.id AND state='waiting') FROM recurring_rules WHERE deleted=0 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RecurringRule{}
	for rows.Next() {
		var r RecurringRule
		var body string
		var id, user, revision int64
		var enabled bool
		var detail string
		var waiting int
		if err := rows.Scan(&id, &user, &body, &revision, &enabled, &detail, &waiting); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			return nil, err
		}
		r.ID, r.UserID, r.Revision, r.Enabled, r.LastError, r.Waiting = id, user, revision, enabled, detail, waiting
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SaveRecurringRule(ctx context.Context, r RecurringRule) (RecurringRule, error) {
	r.LastError = ""
	r.Waiting = 0
	body, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if r.ID == 0 {
		res, err := s.db.ExecContext(ctx, `INSERT INTO recurring_rules(user_id,config,enabled) VALUES(?,?,?)`, r.UserID, string(body), r.Enabled)
		if err != nil {
			return r, err
		}
		r.ID, err = res.LastInsertId()
		r.Revision = 1
		return r, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE recurring_rules SET config=?,enabled=?,revision=revision+1,last_error='' WHERE id=? AND user_id=? AND revision=? AND deleted=0`, string(body), r.Enabled, r.ID, r.UserID, r.Revision)
	if err != nil {
		return r, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return r, err
	}
	if n != 1 {
		return r, errors.New("rule changed or was deleted; reload before saving")
	}
	r.Revision++
	r.LastError = ""
	return r, nil
}

// DeleteRecurringRule never touches an active task, even if its timer won
// the race against deleting this rule.
func (s *Store) DeleteRecurringRule(ctx context.Context, user, id, revision int64, cancelWaiting bool) ([]int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE recurring_rules SET deleted=1,enabled=0,revision=revision+1 WHERE id=? AND user_id=? AND revision=? AND deleted=0`, id, user, revision)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return nil, errors.New("rule changed or was deleted; reload before deleting")
	}
	ids := []int64{}
	if cancelWaiting {
		rows, err := tx.QueryContext(ctx, `UPDATE sell_schedules SET state='cancelled',detail='Recurring rule removed',ended_at=?,revision=revision+1 WHERE rule_id=? AND state='waiting' RETURNING id,date,meal,user_id`, time.Now().Unix(), id)
		if err != nil {
			return nil, err
		}
		var kept []SellSchedule
		for rows.Next() {
			var sc SellSchedule
			if err := rows.Scan(&sc.ID, &sc.Date, &sc.Meal, &sc.UserID); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, sc.ID)
			kept = append(kept, sc)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		for _, sc := range kept {
			if err := keepSchedule(ctx, tx, sc); err != nil {
				return nil, err
			}
		}
	}
	return ids, tx.Commit()
}

func (s *Store) RecurringRuleError(ctx context.Context, id, revision int64, detail string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE recurring_rules SET last_error=? WHERE id=? AND revision=? AND deleted=0`, detail, id, revision)
	return err
}

func (s *Store) KeptMeals(ctx context.Context, user int64) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date,meal FROM planner_kept WHERE user_id=?`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var date, meal string
		if err := rows.Scan(&date, &meal); err != nil {
			return nil, err
		}
		out[date+"|"+meal] = true
	}
	return out, rows.Err()
}

func (s *Store) AllowRecurringMeal(ctx context.Context, user int64, date, meal string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM planner_kept WHERE user_id=? AND date=? AND meal=?`, user, date, meal)
	return err
}

// ApplyRecurringSchedule preserves individual overrides and rejects stale
// generator snapshots if the rule or its schedule changed meanwhile.
func (s *Store) ApplyRecurringSchedule(ctx context.Context, sc SellSchedule, cancel bool) (bool, error) {
	state, detail := "waiting", ""
	if cancel {
		state, detail = "cancelled", "Removed by recurring rule change"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE sell_schedules SET min_paise=?,max_paise=?,starts_at=?,autopilot=?,state=?,detail=?,revision=revision+1,rule_revision=?,retry_at=0,attempts=0,ended_at=? WHERE id=? AND revision=? AND rule_id=? AND state='waiting' AND overridden=0 AND EXISTS(SELECT 1 FROM recurring_rules WHERE id=? AND revision=? AND deleted=0)`, sc.MinPaise, sc.MaxPaise, sc.StartsAt.Unix(), sc.Autopilot, state, detail, sc.RuleRevision, func() int64 {
		if cancel {
			return time.Now().Unix()
		}
		return 0
	}(), sc.ID, sc.Revision, sc.RuleID, sc.RuleID, sc.RuleRevision)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return false, err
	}
	if cancel {
		if err := keepSchedule(ctx, tx, sc); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// keepSchedule is called inside the cancellation transaction so a generator
// cannot recreate a meal between removing its schedule and saving the exception.
func keepSchedule(ctx context.Context, tx *sql.Tx, sc SellSchedule) error {
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO planner_kept(user_id,date,meal) VALUES(?,?,?)`, sc.UserID, sc.Date, sc.Meal)
	return err
}

type RecurringPreview struct {
	New                int `json:"new"`
	StartsNow          int `json:"starts_now"`
	Waiting            int `json:"waiting"`
	PreservedOverrides int `json:"preserved_overrides"`
	Kept               int `json:"kept"`
	Existing           int `json:"existing"`
}
