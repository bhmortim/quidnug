import React from "react";
import { useTrust } from "../hooks/useTrust.js";

/**
 * Visual trust chip — colored green → amber → red based on level.
 *
 *   <TrustBadge observer="alice-id" target="bob-id" domain="demo.home" />
 *
 * Accepts optional `format` prop (defaults to two decimal places) and
 * `threshold` to highlight "passes" vs "fails".
 */
export function TrustBadge({
    observer,
    target,
    domain,
    maxDepth = 5,
    format = (v) => v.toFixed(2),
    threshold,
    className,
    style,
}) {
    const { data, loading, error } = useTrust(observer, target, domain, { maxDepth });

    if (error) {
        return React.createElement(
            "span",
            { className, style: { ...style, color: "#b00" }, title: String(error) },
            "?"
        );
    }
    if (loading || !data) {
        return React.createElement("span", { className, style }, "…");
    }

    const level = data.trustLevel ?? 0;
    const bg =
        threshold !== undefined
            ? level >= threshold ? "#c8e6c9" : "#ffcdd2"
            : level >= 0.7 ? "#c8e6c9"
            : level >= 0.4 ? "#fff9c4"
            : "#ffcdd2";

    return React.createElement(
        "span",
        {
            className,
            title: `depth ${data.pathDepth ?? "?"}`,
            style: {
                display: "inline-block",
                padding: "2px 8px",
                borderRadius: "8px",
                background: bg,
                fontFamily: "monospace",
                fontSize: "0.85em",
                ...style,
            },
        },
        format(level)
    );
}
