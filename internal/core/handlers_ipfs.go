// Package core. handlers_ipfs.go — IPFS pin/get HTTP handlers.
package core

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

func (node *QuidnugNode) PinToIPFSHandler(w http.ResponseWriter, r *http.Request) {
	if !node.IPFSClient.IsAvailable() {
		WriteError(w, http.StatusServiceUnavailable, "IPFS_UNAVAILABLE", "IPFS service is not available")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Failed to read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Request body is empty")
		return
	}

	// Check if content is base64 encoded via Content-Transfer-Encoding header
	var content []byte
	if r.Header.Get("Content-Transfer-Encoding") == "base64" {
		content, err = base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid base64 encoding")
			return
		}
	} else {
		content = body
	}

	cid, err := node.IPFSClient.Pin(r.Context(), content)
	if err != nil {
		if errors.Is(err, ErrIPFSUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "IPFS_UNAVAILABLE", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to pin content to IPFS")
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"cid": cid,
	})
}

// GetFromIPFSHandler retrieves content from IPFS by CID
func (node *QuidnugNode) GetFromIPFSHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cid := vars["cid"]

	if !IsValidCID(cid) {
		WriteError(w, http.StatusBadRequest, "INVALID_CID", "Invalid CID format")
		return
	}

	if !node.IPFSClient.IsAvailable() {
		WriteError(w, http.StatusServiceUnavailable, "IPFS_UNAVAILABLE", "IPFS service is not available")
		return
	}

	content, err := node.IPFSClient.Get(r.Context(), cid)
	if err != nil {
		if errors.Is(err, ErrIPFSUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "IPFS_UNAVAILABLE", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidCID) {
			WriteError(w, http.StatusBadRequest, "INVALID_CID", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve content from IPFS")
		return
	}

	contentType := http.DetectContentType(content)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-IPFS-CID", cid)
	w.Write(content)
}
