package com.quidnug.client;

import java.util.Collections;
import java.util.Map;

/** Base class for every SDK-raised exception. */
public class QuidnugException extends RuntimeException {
    private final Map<String, Object> details;

    public QuidnugException(String message) {
        this(message, Collections.emptyMap());
    }

    public QuidnugException(String message, Map<String, Object> details) {
        super(message);
        this.details = details == null ? Collections.emptyMap() : details;
    }

    public QuidnugException(String message, Throwable cause) {
        super(message, cause);
        this.details = Collections.emptyMap();
    }

    public Map<String, Object> details() { return details; }

    /** Local precondition failed before any network call. */
    public static final class ValidationException extends QuidnugException {
        public ValidationException(String message) { super(message); }
        public ValidationException(String message, Map<String, Object> details) {
            super(message, details);
        }
    }

    /**
     * Node logically rejected a transaction — nonce replay, quorum
     * not met, guardian-set-hash mismatch, etc. (HTTP 409 or select 400s).
     */
    public static final class ConflictException extends QuidnugException {
        public ConflictException(String message, Map<String, Object> details) {
            super(message, details);
        }
    }

    /** HTTP 503 / feature-not-active / bootstrapping. */
    public static final class UnavailableException extends QuidnugException {
        public UnavailableException(String message, Map<String, Object> details) {
            super(message, details);
        }
    }

    /**
     * Transport, unexpected 5xx, malformed response, or any non-envelope
     * shape.
     */
    public static final class NodeException extends QuidnugException {
        private final int statusCode;
        private final String responseBody;

        public NodeException(String message, int statusCode, String responseBody) {
            super(message);
            this.statusCode = statusCode;
            this.responseBody = truncate(responseBody);
        }

        public NodeException(String message, Throwable cause) {
            super(message, cause);
            this.statusCode = 0;
            this.responseBody = null;
        }

        public int statusCode() { return statusCode; }
        public String responseBody() { return responseBody; }

        private static String truncate(String s) {
            if (s == null) return null;
            return s.length() > 500 ? s.substring(0, 500) + "..." : s;
        }
    }

    /** Signature / key derivation / crypto failure. */
    public static final class CryptoException extends QuidnugException {
        public CryptoException(String message) { super(message); }
    }
}
