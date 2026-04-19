package chainlink

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/quidnug/quidnug/pkg/client"
)

// Request matches the Chainlink External Adapter standard:
// https://docs.chain.link/chainlink-nodes/external-adapters/developers
type Request struct {
	// JobRunID is the Chainlink job run ID — echoed back in the response.
	JobRunID string `json:"id"`
	// Data carries the adapter-specific parameters.
	Data RequestData `json:"data"`
}

// RequestData holds the actual query.
type RequestData struct {
	Observer string `json:"observer"`
	Target   string `json:"target"`
	Domain   string `json:"domain"`
	MaxDepth int    `json:"maxDepth"`
}

// Response matches the Chainlink External Adapter standard.
type Response struct {
	JobRunID   string       `json:"jobRunID"`
	Data       ResponseData `json:"data"`
	Result     float64      `json:"result"`      // primary value, for convenience
	StatusCode int          `json:"statusCode"`
	Error      string       `json:"error,omitempty"`
}

// ResponseData is the trust-query payload.
type ResponseData struct {
	TrustLevel float64  `json:"trustLevel"`
	PathDepth  int      `json:"pathDepth"`
	Path       []string `json:"path"`
	Observer   string   `json:"observer"`
	Target     string   `json:"target"`
	Domain     string   `json:"domain"`
}

// Handler returns an http.Handler that serves the External Adapter
// endpoint. Mount at any path (Chainlink defaults to `/`).
func Handler(c *client.Client) http.Handler {
	if c == nil {
		panic("chainlink.Handler: client is required")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, "", "POST required", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "", "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := validate(req.Data); err != nil {
			writeError(w, req.JobRunID, err.Error(), http.StatusBadRequest)
			return
		}
		depth := req.Data.MaxDepth
		if depth <= 0 {
			depth = 5
		}
		tr, err := c.GetTrust(r.Context(), req.Data.Observer, req.Data.Target, req.Data.Domain, depth)
		if err != nil {
			writeError(w, req.JobRunID, err.Error(), http.StatusBadGateway)
			return
		}
		resp := Response{
			JobRunID: req.JobRunID,
			Data: ResponseData{
				TrustLevel: tr.TrustLevel,
				PathDepth:  tr.PathDepth,
				Path:       tr.Path,
				Observer:   tr.Observer,
				Target:     tr.Target,
				Domain:     tr.Domain,
			},
			Result:     tr.TrustLevel,
			StatusCode: http.StatusOK,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func validate(d RequestData) error {
	if d.Observer == "" {
		return errors.New("observer is required")
	}
	if d.Target == "" {
		return errors.New("target is required")
	}
	if d.Domain == "" {
		return errors.New("domain is required")
	}
	return nil
}

func writeError(w http.ResponseWriter, jobID, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{
		JobRunID:   jobID,
		StatusCode: status,
		Error:      fmt.Sprintf("%d: %s", status, msg),
	})
}
