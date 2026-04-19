import React from "react";
import { useGuardianSet } from "../hooks/useGuardianSet.js";

/**
 * Card visualization of a quid's guardian set (QDP-0002).
 *
 *   <GuardianSetCard quidId="..." />
 *
 * Shows threshold, recovery delay, and the list of guardian quids
 * with their weights.
 */
export function GuardianSetCard({ quidId, className, style }) {
    const { data, loading, error } = useGuardianSet(quidId);
    if (error) return React.createElement("div", { className, style }, String(error));
    if (loading) return React.createElement("div", { className, style }, "…");
    if (!data) {
        return React.createElement(
            "div",
            {
                className,
                style: { padding: 12, border: "1px solid #ddd", borderRadius: 8, ...style },
            },
            "no guardian set"
        );
    }

    const guardians = data.guardians ?? [];
    const totalWeight = guardians.reduce(
        (s, g) => s + (g.weight && g.weight > 0 ? g.weight : 1),
        0
    );
    const delayDays = ((data.recoveryDelaySeconds ?? 0) / 86400).toFixed(1);

    return React.createElement(
        "div",
        {
            className,
            style: {
                padding: 12,
                border: "1px solid #ddd",
                borderRadius: 8,
                fontFamily: "system-ui",
                ...style,
            },
        },
        React.createElement(
            "div",
            { style: { fontSize: 14, color: "#555" } },
            `threshold ${data.threshold} of ${totalWeight} · recovery delay ${delayDays}d`
        ),
        React.createElement(
            "ul",
            { style: { margin: "8px 0 0", padding: 0, listStyle: "none" } },
            ...guardians.map((g, i) =>
                React.createElement(
                    "li",
                    {
                        key: g.quid ?? i,
                        style: {
                            padding: "4px 8px",
                            background: i % 2 === 0 ? "#f7f7f7" : "transparent",
                            fontFamily: "monospace",
                            fontSize: 13,
                            display: "flex",
                            justifyContent: "space-between",
                        },
                    },
                    React.createElement("span", null, g.quid),
                    React.createElement(
                        "span",
                        { style: { color: "#888" } },
                        `w=${g.weight ?? 1}`
                    )
                )
            )
        )
    );
}
