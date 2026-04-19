import React from "react";
import { QuidnugProvider, useTrust, useQuid } from "@quidnug/react";

/**
 * Example: gate a destructive button behind a relational-trust
 * threshold check. The button stays disabled until we can verify
 * the signer trusts the target at ≥ 0.7.
 */
export function TrustGatedAction({ target, onApprove }) {
    const { quid } = useQuid();
    const { data, loading } = useTrust(quid?.id, target, "company.home");

    if (!quid) return <em>sign in first</em>;
    if (loading) return <button disabled>checking trust…</button>;

    const level = data?.trustLevel ?? 0;
    const approved = level >= 0.7;

    return (
        <div>
            <p>
                your trust in <code>{target}</code>: <b>{level.toFixed(3)}</b>
            </p>
            <button onClick={onApprove} disabled={!approved}>
                {approved ? "Approve" : `Need ≥ 0.7 trust (have ${level.toFixed(2)})`}
            </button>
        </div>
    );
}
