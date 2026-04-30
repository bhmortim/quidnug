// Package core. response.go — pagination and JSON response helpers.
package core

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Pagination constants
const (
	DefaultPaginationLimit = 50
	MaxPaginationLimit     = 1000
)

// PaginationParams holds parsed pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

// PaginationMeta contains pagination metadata for responses
type PaginationMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

// ParsePaginationParams extracts limit and offset from query parameters
// with validation and capping to maxLimit
func ParsePaginationParams(r *http.Request, defaultLimit, maxLimit int) PaginationParams {
	params := PaginationParams{
		Limit:  defaultLimit,
		Offset: 0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			if limit > 0 {
				params.Limit = limit
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			if offset >= 0 {
				params.Offset = offset
			}
		}
	}

	if params.Limit > maxLimit {
		params.Limit = maxLimit
	}

	return params
}

// paginateSlice is a helper to paginate a generic slice
// Returns the paginated slice and total count
func paginateSlice[T any](items []T, params PaginationParams) ([]T, int) {
	total := len(items)

	if params.Offset >= total {
		return []T{}, total
	}

	end := params.Offset + params.Limit
	if end > total {
		end = total
	}

	return items[params.Offset:end], total
}

// encodeEnvelope encodes v to w and logs (without panicking) if
// the write fails. Almost all encode failures here come from the
// client disconnecting mid-response; that's not a server fault
// and shouldn't generate noise above debug. We don't try to
// rescue: by the time Encode fails the response has typically
// committed bytes already, so writing a different status would
// just produce a malformed wire response.
func encodeEnvelope(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// logger may be nil in tests that exercise the
		// helpers without initializing the package. Be safe.
		if logger != nil {
			logger.Debug("response: encode failed", "err", err)
		}
	}
}

// WriteSuccess writes a successful JSON response with envelope
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	encodeEnvelope(w, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// WriteSuccessWithStatus writes a successful JSON response with custom status code
func WriteSuccessWithStatus(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	w.WriteHeader(status)
	encodeEnvelope(w, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// WriteError writes an error JSON response with envelope
func WriteError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	w.WriteHeader(status)
	encodeEnvelope(w, map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// WriteFieldError writes a field validation error response
func WriteFieldError(w http.ResponseWriter, code string, message string, fields []string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-API-Version", "1.0")
	w.WriteHeader(http.StatusBadRequest)
	encodeEnvelope(w, map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"fields":  fields,
		},
	})
}
