import React from "react";
import {
    QuidnugProvider,
    TrustBadge,
    TrustPath,
    GuardianSetCard,
    useStream,
    useQuid,
    useGrantTrust,
} from "@quidnug/react";

/**
 * Example: trust dashboard for a small team.
 *
 * - Top: grid of every pairwise trust relationship as colored badges.
 * - Middle: a named trust-path view for the currently-selected pair.
 * - Bottom: audit log of recent events on the active user quid.
 */
export function TrustDashboard({ team }) {
    return (
        <QuidnugProvider node="https://node.example.com" defaultDomain="company.home">
            <h1>Team Trust</h1>

            <TrustMatrix team={team} />

            <h2>Your audit log</h2>
            <YourAuditLog />

            <h2>Guardians</h2>
            <YourGuardianSet />
        </QuidnugProvider>
    );
}

function TrustMatrix({ team }) {
    return (
        <table>
            <thead>
                <tr>
                    <th></th>
                    {team.map((p) => <th key={p.id}>{p.name}</th>)}
                </tr>
            </thead>
            <tbody>
                {team.map((observer) => (
                    <tr key={observer.id}>
                        <th>{observer.name}</th>
                        {team.map((target) => (
                            <td key={target.id}>
                                {observer.id === target.id ? "—" : (
                                    <TrustBadge
                                        observer={observer.id}
                                        target={target.id}
                                        domain="company.home"
                                    />
                                )}
                            </td>
                        ))}
                    </tr>
                ))}
            </tbody>
        </table>
    );
}

function YourAuditLog() {
    const { quid } = useQuid();
    const { data, loading } = useStream(quid?.id, { limit: 10 });
    if (!quid) return <em>sign in first</em>;
    if (loading) return "loading…";
    return (
        <ul>
            {data?.events?.map((e) => (
                <li key={e.sequence}>
                    #{e.sequence} {e.eventType} — {new Date(e.timestamp * 1000).toISOString()}
                </li>
            ))}
        </ul>
    );
}

function YourGuardianSet() {
    const { quid } = useQuid();
    if (!quid) return <em>sign in first</em>;
    return <GuardianSetCard quidId={quid.id} />;
}
