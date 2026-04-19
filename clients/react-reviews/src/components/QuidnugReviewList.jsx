import React, { useMemo } from "react";
import { useReviews } from "../hooks/useReviews.js";

/**
 * <QuidnugReviewList product="..." topic="..." />
 *
 * Renders reviews sorted by your-trust-weight. Accepts optional
 * pre-computed contributions to avoid fetching twice when used
 * inside <QuidnugReviewPanel>.
 */
export function QuidnugReviewList({
    product,
    topic,
    contributions,
    sort = "weight",
    limit = 20,
}) {
    const { data: reviews, loading, error } = useReviews(product, topic, { limit });

    const weightMap = useMemo(() => {
        if (!contributions) return new Map();
        return new Map(contributions.map((c) => [c.reviewerQuid, c.weight]));
    }, [contributions]);

    const sorted = useMemo(() => {
        const copy = [...reviews];
        const byWeight = (a, b) => (weightMap.get(b.creator) ?? 0) - (weightMap.get(a.creator) ?? 0);
        const byRecent = (a, b) => (b.timestamp ?? 0) - (a.timestamp ?? 0);
        const byHigh = (a, b) => (b.payload?.rating ?? 0) - (a.payload?.rating ?? 0);
        const byLow = (a, b) => (a.payload?.rating ?? 0) - (b.payload?.rating ?? 0);
        switch (sort) {
            case "recent":      copy.sort(byRecent); break;
            case "rating-high": copy.sort(byHigh); break;
            case "rating-low":  copy.sort(byLow); break;
            case "weight":
            default:            copy.sort(byWeight);
        }
        return copy;
    }, [reviews, sort, weightMap]);

    if (loading) return <div style={{ color: "#888" }}>Loading reviews…</div>;
    if (error) return <div style={{ color: "#c0392b" }}>{String(error.message ?? error)}</div>;
    if (sorted.length === 0) return <em style={{ color: "#888" }}>No reviews yet</em>;

    return (
        <div style={{ display: "flex", flexDirection: "column", gap: 12, marginTop: 12 }}>
            {sorted.map((ev) => (
                <ReviewRow key={`${ev.subjectId}-${ev.sequence}`}
                           ev={ev} weight={weightMap.get(ev.creator)} />
            ))}
        </div>
    );
}

function ReviewRow({ ev, weight }) {
    const payload = ev.payload ?? {};
    const rating = payload.rating;
    const contributing = weight && weight > 0.1;
    return (
        <div style={{
            padding: "10px 12px",
            borderRadius: 6,
            background: "#f7f7f7",
            borderLeft: `3px solid ${contributing ? "#2E8B57" : "#ccc"}`,
            opacity: contributing ? 1 : 0.55,
            fontSize: 14,
        }}>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <span style={{ color: "#F9A825", fontWeight: 600 }}>★ {rating}</span>
                {payload.title && <strong>{payload.title}</strong>}
                <span style={{
                    fontSize: 11, color: contributing ? "#2E8B57" : "#999",
                    fontFamily: "monospace",
                }}>
                    {weight != null ? `weight ${weight.toFixed(2)}` : "outside your trust"}
                </span>
            </div>
            <div style={{ fontFamily: "monospace", fontSize: 11, color: "#888", margin: "4px 0" }}>
                by {String(ev.creator ?? "").slice(0, 10)}…
            </div>
            <div>{payload.bodyMarkdown}</div>
        </div>
    );
}
