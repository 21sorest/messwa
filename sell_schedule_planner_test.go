package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"messwa/internal/config"
	"messwa/internal/portal"
	"messwa/internal/store"
)

func TestPlannerScheduleEditsAreVersionedAndFrozen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f := newSellFixture(t, path)
		f.cfg.MCP.SellTasks = true
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := f.manager(ctx)
		slot, _ := findSlot(f.cfg, f.date, "lunch")
		at := slot.Start.Add(-25 * time.Minute)
		id, got, err := m.PlanSchedule(ctx, 0, 0, 0, f.date, "lunch", 30, 60, at, false)
		if err != nil || !got.Equal(at) {
			t.Fatalf("plan %v %v", got, err)
		}
		sc := f.schedule(id)
		if _, err := f.cfg.Update(config.Settings{"sell.list_at": []byte(`-300`)}); err != nil {
			t.Fatal(err)
		}
		if _, _, err = m.PlanSchedule(ctx, 0, id, sc.Revision, f.date, "lunch", 35, 70, at, true); err != nil {
			t.Fatal(err)
		}
		if _, _, err = m.PlanSchedule(ctx, 0, id, sc.Revision, f.date, "lunch", 40, 80, at, false); err == nil {
			t.Fatal("stale edit overwrote a newer schedule")
		}
		if err = m.CancelPlannedSchedule(ctx, id, sc.Revision); err == nil {
			t.Fatal("stale cancellation removed edited schedule")
		}
		if _, _, err = m.PlanSchedule(ctx, 99, id, f.schedule(id).Revision, f.date, "lunch", 40, 80, at, false); err == nil {
			t.Fatal("another user edited the schedule")
		}
		time.Sleep(time.Until(at) + time.Second)
		synctest.Wait()
		task := f.activeTask("lunch")
		if task == nil || task.MinPaise != 3500 || task.MaxPaise != 7000 || !task.Autopilot {
			t.Fatalf("task %+v schedule %+v", task, f.schedule(id))
		}
		if !f.schedule(id).StartsAt.Equal(at) {
			t.Fatal("defaults changed the saved posting time")
		}
		cancel()
		synctest.Wait()
	})
}

func TestPlannerScheduleTemporaryPortalFailureRetries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f := newSellFixture(t, path)
		f.cfg.MCP.SellTasks = true
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := f.manager(ctx)
		id, at, err := m.StartOrSchedule(ctx, 0, f.date, "lunch", 40, 60, false)
		if err != nil {
			t.Fatal(err)
		}
		original := m.portalFor
		m.portalFor = func(context.Context, int64) (mealPortal, error) { return nil, portal.HTTPError{Status: 503} }
		time.Sleep(time.Until(at) + time.Second)
		synctest.Wait()
		sc := f.schedule(id)
		if sc.State != store.ScheduleWaiting || sc.Attempts != 1 || sc.RetryAt.IsZero() || sc.Detail == "" {
			t.Fatalf("retry %+v", sc)
		}
		m.portalFor = original
		time.Sleep(time.Until(sc.RetryAt) + time.Second)
		synctest.Wait()
		if sc = f.schedule(id); sc.State != store.ScheduleStarted || sc.TaskID == 0 {
			t.Fatalf("not started after recovery: %+v", sc)
		}
		cancel()
		synctest.Wait()
	})
}

func TestPlannerScheduleInvalidRegistrationDoesNotRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f := newSellFixture(t, path)
		f.cfg.MCP.SellTasks = true
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := f.manager(ctx)
		id, at, err := m.StartOrSchedule(ctx, 0, f.date, "lunch", 40, 60, false)
		if err != nil {
			t.Fatal(err)
		}
		m.portalFor = func(context.Context, int64) (mealPortal, error) { return nil, errors.New("meal is cancelled or used") }
		time.Sleep(time.Until(at) + time.Minute)
		synctest.Wait()
		if sc := f.schedule(id); sc.State != store.ScheduleFailed || sc.Attempts != 0 || sc.Detail != "meal is cancelled or used" {
			t.Fatalf("invalid registration %+v", sc)
		}
		if len(f.conn.messages()) != 0 {
			t.Fatal("invalid registration posted an offer")
		}
		cancel()
		synctest.Wait()
	})
}

func TestPlannerScheduleDisconnectedRetriesAreBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f := newSellFixture(t, path)
		f.cfg.MCP.SellTasks = true
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := f.manager(ctx)
		id, at, err := m.StartOrSchedule(ctx, 0, f.date, "lunch", 40, 60, false)
		if err != nil {
			t.Fatal(err)
		}
		m.connected = func(time.Duration) bool { return false }
		time.Sleep(time.Until(at) + 9*time.Minute)
		synctest.Wait()
		if sc := f.schedule(id); sc.State != store.ScheduleFailed || sc.Attempts != 5 {
			t.Fatalf("unbounded retries: %+v", sc)
		}
		if f.activeTask("lunch") != nil || len(f.conn.messages()) != 0 {
			t.Fatal("disconnected schedule started a sale")
		}
		cancel()
		synctest.Wait()
	})
}

func TestPlannerScheduleRecoversInterruptedClaim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f := newSellFixture(t, path)
		f.cfg.MCP.SellTasks = true
		ctx, stop := context.WithCancel(t.Context())
		id, at, err := f.manager(ctx).StartOrSchedule(ctx, 0, f.date, "lunch", 40, 60, false)
		if err != nil {
			t.Fatal(err)
		}
		stop()
		synctest.Wait()
		if ok, err := f.st.EndSellSchedule(t.Context(), id, store.ScheduleStarting, 0, "", time.Now()); err != nil || !ok {
			t.Fatalf("claim %v %v", ok, err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if err := f.manager(ctx).Resume(ctx, true); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Until(at) + time.Minute)
		synctest.Wait()
		sc := f.schedule(id)
		if sc.State != store.ScheduleStarted || sc.TaskID == 0 || f.activeTask("lunch") == nil {
			t.Fatalf("lost interrupted schedule: %+v", sc)
		}
		cancel()
		synctest.Wait()
	})
}
