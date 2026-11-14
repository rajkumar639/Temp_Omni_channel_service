// Package router wires HTTP routes declared in routes.toml to their
// handler functions using only the standard library's net/http ServeMux.
package router

import (
	"encoding/json"
	"net/http"

	"omniChannelservice/service"
	"omniChannelservice/tomlloader"
	"omniChannelservice/util"
)

// New creates a *http.ServeMux with all routes from the config registered.
// Unknown handler names fall back to a 501 Not Implemented response.
func New(routes []tomlloader.RouteEntry, log *util.Logger, notifier *service.NotificationService) *http.ServeMux {
	mux := http.NewServeMux()

	for _, r := range routes {
		h := resolve(r.Handler, log, notifier)
		pattern := r.Method + " " + r.Path // Go 1.22+ method+pattern routing
		mux.HandleFunc(pattern, h)
		log.Info("route registered: %-6s %s → %s", r.Method, r.Path, r.Handler)
	}

	return mux
}

// resolve maps a handler name string to an actual http.HandlerFunc.
func resolve(name string, log *util.Logger, notifier *service.NotificationService) http.HandlerFunc {
	switch name {
	case "HealthHandler":
		return healthHandler(log)
	case "GetUsersHandler":
		return getUsersHandler(log)
	case "CreateUserHandler":
		return createUserHandler(log)
	case "SendNotificationHandler":
		return sendNotificationHandler(log, notifier)
	default:
		log.Warn("no handler found for %q, using 501 stub", name)
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not implemented: "+name, http.StatusNotImplemented)
		}
	}
}

// ── handlers ─────────────────────────────────────────────────────────────────

func healthHandler(log *util.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("health check from %s", r.RemoteAddr)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func getUsersHandler(log *util.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("get users request")
		// TODO: query integration.DB and return real rows
		writeJSON(w, http.StatusOK, []map[string]any{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		})
	}
}

func createUserHandler(log *util.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("create user request")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// TODO: insert via integration.DB
		writeJSON(w, http.StatusCreated, map[string]string{"result": "created"})
	}
}

func sendNotificationHandler(log *util.Logger, notifier *service.NotificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Kind     string `json:"kind"`
			To       string `json:"to,omitempty"`
			Message  string `json:"message,omitempty"`
			Subject  string `json:"subject,omitempty"`
			Body     string `json:"body,omitempty"`
			Messages []struct {
				To      string `json:"to"`
				Subject string `json:"subject,omitempty"`
				Body    string `json:"body,omitempty"`
				Message string `json:"message,omitempty"`
			} `json:"messages,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Kind == "" {
			http.Error(w, "missing kind", http.StatusBadRequest)
			return
		}

		payload := map[string]any{
			"subject": req.Subject,
			"body":    req.Body,
		}

		if len(req.Messages) > 0 {
			messages := make([]any, 0, len(req.Messages))
			for _, m := range req.Messages {
				item := map[string]any{"to": m.To}
				if req.Kind == "email" {
					item["subject"] = firstNonEmpty(m.Subject, req.Subject)
					item["body"] = firstNonEmpty(m.Body, m.Message, req.Body)
				} else {
					item["message"] = firstNonEmpty(m.Message, m.Body)
				}
				messages = append(messages, item)
			}
			payload["messages"] = messages
		} else {
			payload["to"] = req.To
			if req.Kind == "email" {
				payload["subject"] = req.Subject
				payload["body"] = req.Body
			} else {
				payload["message"] = req.Message
			}
		}

		if err := notifier.SendAsync(req.Kind, payload); err != nil {
			http.Error(w, "notification queue full", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "Accepted Request",
			"message": "Notification send request accepted",
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
