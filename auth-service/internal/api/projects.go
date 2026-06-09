package api

import (
	"encoding/json"
	"net/http"
)

type projectResponse struct {
	Resource  string `json:"resource"`
	Operation string `json:"operation"`
}

func ProjectsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var resp projectResponse

	switch r.Method {
	case http.MethodGet:
		resp = projectResponse{
			Resource:  "projects",
			Operation: "read",
		}

	case http.MethodPost:
		resp = projectResponse{
			Resource:  "projects",
			Operation: "create",
		}

	case http.MethodDelete:
		resp = projectResponse{
			Resource:  "projects",
			Operation: "purge",
		}

	default:
		http.Error(
			w,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed,
		)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
