import React, { createContext, useContext, useMemo } from "react";
import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";

/**
 * QuidnugContext shape:
 *   { client: QuidnugClient, quid: Quid | null, setQuid: (q) => void, defaultDomain: string }
 *
 * The context intentionally keeps things simple — no loading state,
 * no automatic identity bootstrap. Apps that need those layers should
 * wrap QuidnugProvider with their own identity-management component.
 */
const QuidnugContext = createContext(null);

export function QuidnugProvider({
    node,
    initialQuid = null,
    defaultDomain = "default",
    maxRetries,
    retryBaseDelayMs,
    children,
}) {
    const [quid, setQuid] = React.useState(initialQuid);

    const client = useMemo(() => {
        const opts = { defaultNode: node };
        if (maxRetries !== undefined) opts.maxRetries = maxRetries;
        if (retryBaseDelayMs !== undefined) opts.retryBaseDelayMs = retryBaseDelayMs;
        return new QuidnugClient(opts);
    }, [node, maxRetries, retryBaseDelayMs]);

    const value = useMemo(
        () => ({ client, quid, setQuid, defaultDomain }),
        [client, quid, defaultDomain]
    );

    return React.createElement(
        QuidnugContext.Provider,
        { value },
        children
    );
}

/** Hook returning `{ client, quid, setQuid, defaultDomain }`. */
export function useQuidnug() {
    const ctx = useContext(QuidnugContext);
    if (ctx === null) {
        throw new Error("useQuidnug must be called inside <QuidnugProvider>");
    }
    return ctx;
}
