package dashboard

import (
	"messwa/internal/accounts"
	"messwa/internal/mcpserver"
	"messwa/internal/store"
	"net/http"
)

func recurringRoutes(mux *http.ServeMux, srv *mcpserver.Server) {
	wrap := func(fn http.HandlerFunc) http.Handler {
		return requireRole(accounts.Member, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !srv.Scoped() && !accounts.UserFrom(r.Context()).IsAdmin() {
				jsonError(w, "admins only", 403)
				return
			}
			fn(w, r)
		}))
	}
	mux.Handle("GET /api/recurring-rules", wrap(func(w http.ResponseWriter, r *http.Request) {
		rules, err := srv.RecurringRules(r.Context(), accounts.UserFrom(r.Context()).ID)
		if err != nil {
			jsonError(w, "Could not read recurring rules", 500)
			return
		}
		writeJSON(w, r, rules)
	}))
	mux.Handle("POST /api/recurring-rules", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Action string              `json:"action"`
			Rule   store.RecurringRule `json:"rule"`
			Apply  bool                `json:"apply_waiting"`
			Cancel bool                `json:"cancel_waiting"`
			Date   string              `json:"date"`
			Meal   string              `json:"meal"`
		}
		if !readJSON(w, r, &in) {
			return
		}
		if ((in.Action == "save" || in.Action == "preview") && in.Rule.Enabled) || in.Action == "allow" {
			if _, err := srv.SellControl(); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		control, err := srv.RecurringControl()
		if err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		user := accounts.UserFrom(r.Context()).ID
		switch in.Action {
		case "preview":
			out, err := control.PreviewRecurringRule(r.Context(), user, in.Rule, in.Apply)
			if err != nil {
				jsonError(w, err.Error(), 400)
				return
			}
			writeJSON(w, r, out)
		case "save":
			out, err := control.SaveRecurringRule(r.Context(), user, in.Rule, in.Apply)
			if err != nil {
				jsonError(w, err.Error(), 400)
				return
			}
			writeJSON(w, r, out)
		case "delete":
			if err := control.DeleteRecurringRule(r.Context(), user, in.Rule.ID, in.Rule.Revision, in.Cancel); err != nil {
				jsonError(w, err.Error(), 409)
				return
			}
			writeJSON(w, r, struct{}{})
		case "allow":
			if err := control.AllowRecurringMeal(r.Context(), user, in.Date, in.Meal); err != nil {
				jsonError(w, err.Error(), 400)
				return
			}
			writeJSON(w, r, struct{}{})
		default:
			jsonError(w, "Unknown recurring rule action", 400)
		}
	}))
}
