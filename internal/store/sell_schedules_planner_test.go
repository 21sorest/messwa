package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestScheduleTaskLinkCommitsTogether(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := t.Context()
	now := time.Now()
	id, err := s.CreateSellSchedule(ctx, SellSchedule{Date: "2026-11-16", Meal: "lunch", UserID: 7, MinPaise: 3000, MaxPaise: 6000, StartsAt: now, EndsAt: now.Add(time.Hour), CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.EndSellSchedule(ctx, id, ScheduleStarting, 0, "", now); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	if _, err := s.CreateSellSchedule(ctx, SellSchedule{Date: "2026-11-16", Meal: "lunch", MinPaise: 3000, MaxPaise: 6000, StartsAt: now, EndsAt: now.Add(time.Hour), CreatedAt: now}); err == nil {
		t.Fatal("duplicated an in-flight schedule")
	}
	task, err := s.CreateSellTask(ctx, SellTask{Date: "2026-11-16", Meal: "lunch", UserID: 7, MinPaise: 3000, MaxPaise: 6000, CreatedAt: now, EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverSellSchedule(ctx, id); err != nil {
		t.Fatal(err)
	}
	sc, err := s.GetSellSchedule(ctx, id)
	if err != nil || sc.TaskID != task || sc.State != ScheduleStarted {
		t.Fatalf("link %+v %v", sc, err)
	}
	if ok, err := s.CancelPlannedSchedule(ctx, id, sc.Revision); err != nil || ok {
		t.Fatalf("cancelled in-flight schedule %v %v", ok, err)
	}
	if _, err := s.CreateSellTask(ctx, SellTask{Date: "2026-11-16", Meal: "lunch", MinPaise: 3000, MaxPaise: 6000, CreatedAt: now, EndsAt: now.Add(time.Hour)}); err == nil {
		t.Fatal("created duplicate task after recovery")
	}
}

func TestScheduleClaimUsesCurrentRevisionAndTime(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := t.Context()
	now := time.Now()
	at := now.Add(time.Minute)
	id, err := s.CreateSellSchedule(ctx, SellSchedule{Date: "2026-11-16", Meal: "lunch", MinPaise: 3000, MaxPaise: 6000, StartsAt: at, EndsAt: at.Add(time.Hour), CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	sc, _ := s.GetSellSchedule(ctx, id)
	oldRevision := sc.Revision
	sc.StartsAt = at.Add(time.Minute)
	if ok, err := s.UpdateSellSchedule(ctx, *sc); err != nil || !ok {
		t.Fatalf("edit %v %v", ok, err)
	}
	if ok, err := s.ClaimSellSchedule(ctx, id, oldRevision, at); err != nil || ok {
		t.Fatalf("old timer claimed edited schedule: %v %v", ok, err)
	}
	sc, _ = s.GetSellSchedule(ctx, id)
	if ok, err := s.ClaimSellSchedule(ctx, id, sc.Revision, at); err != nil || ok {
		t.Fatalf("new timer started early: %v %v", ok, err)
	}
	if ok, err := s.ClaimSellSchedule(ctx, id, sc.Revision, sc.StartsAt); err != nil || !ok {
		t.Fatalf("due timer could not claim: %v %v", ok, err)
	}
	if ok, err := s.CancelPlannedSchedule(ctx, id, sc.Revision); err != nil || ok {
		t.Fatalf("cancel won after start claimed: %v %v", ok, err)
	}
}

func TestLegacyScheduleRecoveryFindsItsExistingTask(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := t.Context()
	now := time.Now()
	id, err := s.CreateSellSchedule(ctx, SellSchedule{Date: "2026-11-16", Meal: "lunch", MinPaise: 3000, MaxPaise: 6000, StartsAt: now, EndsAt: now.Add(time.Hour), CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.EndSellSchedule(ctx, id, ScheduleStarted, 0, "", now); err != nil || !ok {
		t.Fatal(err)
	}
	task, err := s.CreateSellTask(ctx, SellTask{Date: "2026-11-16", Meal: "lunch", MinPaise: 3000, MaxPaise: 6000, CreatedAt: now, EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverSellSchedule(ctx, id); err != nil {
		t.Fatal(err)
	}
	sc, err := s.GetSellSchedule(ctx, id)
	if err != nil || sc.TaskID != task || sc.State != ScheduleStarted {
		t.Fatalf("legacy recovery %+v %v", sc, err)
	}
}
