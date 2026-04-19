package client

import (
	"errors"
	"fmt"
)

// Error is the interface all SDK-raised errors implement.
//
// Use errors.As(err, new(client.ValidationError)) etc. to branch on
// specific error subtypes. errors.Is(err, client.ErrSDK) matches any
// SDK error.
type Error interface {
	error
	// Code returns the server-provided error code (e.g. "NONCE_REPLAY")
	// when available, or a synthetic code for client-side errors.
	Code() string
	// sdk tags this as an SDK error so the sentinel `errors.Is` works.
	sdk()
}

// ErrSDK is a sentinel; errors.Is(err, ErrSDK) matches any Error.
var ErrSDK = errors.New("quidnug client error")

type baseError struct {
	msg     string
	code    string
	details map[string]any
}

func (e *baseError) Error() string { return e.msg }
func (e *baseError) Code() string  { return e.code }
func (e *baseError) sdk()          {}
func (e *baseError) Is(target error) bool {
	return target == ErrSDK
}

// ValidationError is raised when a local precondition fails before any
// network activity. Safe to retry with corrected input.
type ValidationError struct{ baseError }

// ConflictError is raised when the node rejects a transaction for
// logical reasons: nonce replay, guardian-set-hash mismatch, quorum
// not met, duplicate, already exists. Retrying without changes will
// not succeed.
type ConflictError struct{ baseError }

// UnavailableError is raised on HTTP 503 or for feature-gated
// endpoints whose activation fork has not yet fired.
type UnavailableError struct{ baseError }

// NodeError wraps network/transport failures and unexpected HTTP
// errors. Includes the status code and a truncated response body for
// debugging.
type NodeError struct {
	baseError
	StatusCode   int
	ResponseBody string
}

// CryptoError is raised for signature / key derivation failures.
type CryptoError struct{ baseError }

func newValidationError(msg string) *ValidationError {
	return &ValidationError{baseError{msg: msg, code: "VALIDATION"}}
}

func newConflictError(msg, code string, details map[string]any) *ConflictError {
	return &ConflictError{baseError{msg: msg, code: code, details: details}}
}

func newUnavailableError(msg, code string, details map[string]any) *UnavailableError {
	return &UnavailableError{baseError{msg: msg, code: code, details: details}}
}

func newNodeError(msg string, status int, body string) *NodeError {
	return &NodeError{
		baseError:    baseError{msg: msg, code: "TRANSPORT"},
		StatusCode:   status,
		ResponseBody: truncate(body, 500),
	}
}

func newCryptoError(msg string) *CryptoError {
	return &CryptoError{baseError{msg: msg, code: "CRYPTO"}}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// serverErrorToSDKError maps a parsed error envelope to the right SDK
// error subtype based on the status code and the server's error.code.
func serverErrorToSDKError(status int, code, message string, details map[string]any) Error {
	if message == "" {
		message = fmt.Sprintf("HTTP %d", status)
	}
	switch {
	case status == 503 || isUnavailableCode(code):
		return newUnavailableError(message, code, details)
	case status == 409 || isConflictCode(code):
		return newConflictError(message, code, details)
	case status >= 400 && status < 500:
		return &ValidationError{baseError{msg: message, code: code, details: details}}
	default:
		return newNodeError(message, status, "")
	}
}

func isConflictCode(code string) bool {
	switch code {
	case "NONCE_REPLAY",
		"GUARDIAN_SET_MISMATCH",
		"QUORUM_NOT_MET",
		"VETOED",
		"INVALID_SIGNATURE",
		"FORK_ALREADY_ACTIVE",
		"DUPLICATE",
		"ALREADY_EXISTS",
		"INVALID_STATE_TRANSITION":
		return true
	}
	return false
}

func isUnavailableCode(code string) bool {
	switch code {
	case "FEATURE_NOT_ACTIVE", "NOT_READY", "BOOTSTRAPPING":
		return true
	}
	return false
}
