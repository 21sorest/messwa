package mcpserver

import (
	"context"
	"errors"
	"messwa/internal/store"
)

type RecurringControl interface {
	PreviewRecurringRule(context.Context, int64, store.RecurringRule, bool) (store.RecurringPreview, error)
	SaveRecurringRule(context.Context, int64, store.RecurringRule, bool) (store.RecurringRule, error)
	DeleteRecurringRule(context.Context, int64, int64, int64, bool) error
	AllowRecurringMeal(context.Context, int64, string, string) error
}

func (s *Server) RecurringControl() (RecurringControl, error) {
	control, ok := s.sell.(RecurringControl)
	if !ok {
		return nil, errors.New("the live service is required to change recurring rules")
	}
	return control, nil
}

func (s *Server) RecurringRules(ctx context.Context, user int64) ([]store.RecurringRule, error) {
	all, err := s.st.RecurringRules(ctx)
	if err != nil {
		return nil, err
	}
	out := []store.RecurringRule{}
	for _, r := range all {
		if r.UserID == user {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Server) KeptMeals(ctx context.Context, user int64) (map[string]bool, error) {
	return s.st.KeptMeals(ctx, user)
}
