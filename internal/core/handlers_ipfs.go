// Package core. handlers_ipfs.go — IPFS pin/get HTTP handlers.
package core

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/quidnug/quidnug/internal/ipfsclient"
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
		if errors.Is(err, ipfsclient.ErrIPFSUnavailable) {
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

	if !ipfsclient.IsValidCID(cid) {
		WriteError(w, http.StatusBadRequest, "INVALID_CID", "Invalid CID format")
		return
	}

	if !node.IPFSClient.IsAvailable() {
		WriteError(w, http.StatusServiceUnavailable, "IPFS_UNAVAILABLE", "IPFS service is not available")
		return
	}

	content, err := node.IPFSClient.Get(r.Context(), cid)
	if err != nil {
		if errors.Is(err, ipfsclient.ErrIPFSUnavailable) {
			WriteError(w, http.StatusServiceUnavailable, "IPFS_UNAVAILABLE", err.Error())
			return
		}
		if errors.Is(err, ipfsclient.ErrInvalidCID) {
			WriteError(w, http.StatusBadRequest, "INVALID_CID", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve content from IPFS")
		return
	}

	// IPFS payloads are operator-untrusted: a peer can pin a CID
	// whose content is HTML/SVG/JS designed to execute in a
	// browser session bound to this node's origin. We defang
	// the response to prevent reflected-XSS:
	//
	//   * Content-Type forced to application/octet-stream so the
	//     browser does not parse the body as a renderable type.
	//   * X-Content-Type-Options: nosniff prevents Chrome/Edge
	//     from MIME-sniffing back to text/html.
	//   * Content-Disposition: attachment forces a download
	//     instead of inline render. The CID is the suggested
	//     filename so operators can trace the artifact.
	//   * Content-Security-Policy: default-src 'none' is a belt-
	//     and-suspenders guard if a future code path serves this
	//     inline.
	//
	// Callers that need the raw bytes (CLI, SDK) read them as
	// bytes regardless of headers. Browser rendering is the
	// only attack surface.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+cid+".bin\"")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("X-IPFS-CID", cid)
	if _, err := w.Write(content); err != nil {
		// Best-effort: client likely disconnected. Don't 500
		// because headers are already sent.
		logger.Debug("ipfs get: write body failed", "err", err)
	}
}
