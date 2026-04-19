import React from "react";
import { useTrustWeightedRating } from "../hooks/useTrustWeightedRating.js";
import { useReviews } from "../hooks/useReviews.js";
import { QuidnugStars } from "./QuidnugStars.jsx";
import { QuidnugReviewList } from "./QuidnugReviewList.jsx";
import { QuidnugWriteReview } from "./QuidnugWriteReview.jsx";

/**
 * <QuidnugReviewPanel product="..." topic="..." showWrite />
 *
 * Full review experience for a product page: headline trust-weighted
 * rating at top, review list below, inline write form at the bottom
 * (if showWrite is set).
 */
export function QuidnugReviewPanel({ product, topic, showWrite = false }) {
    const rating = useTrustWeightedRating(product, topic);

    return (
        <div style={{
            border: "1px solid #e0e0e0",
            borderRadius: 8,
            padding: 16,
            fontFamily: "system-ui, sans-serif",
            color: "#222",
        }}>
            <Header result={rating.data} loading={rating.loading} error={rating.error} />
            <QuidnugReviewList
                product={product}
                topic={topic}
                contributions={rating.data?.contributions}
            />
            {showWrite && <QuidnugWriteReview product={product} topic={topic} onSuccess={rating.refetch} />}
        </div>
    );
}

function Header({ result, loading, error }) {
    if (loading && !result) return <div style={{ color: "#888" }}>Computing your weighted rating…</div>;
    if (error) return <div style={{ color: "#c0392b" }}>Rating unavailable: {String(error.message ?? error)}</div>;

    const rating = result?.rating;
    return (
        <div>
            <h3 style={{ margin: "0 0 4px 0", fontSize: 13, color: "#666", fontWeight: 500 }}>
                Trust-weighted rating (your view)
            </h3>
            {rating == null ? (
                <em style={{ color: "#888" }}>not enough trusted reviews for you yet</em>
            ) : (
                <>
                    <span style={{ fontSize: 36, fontWeight: 700, color: "#0B1D3A" }}>
                        {rating.toFixed(1)}
                    </span>
                    <span style={{ fontSize: 14, color: "#666", marginLeft: 8 }}>
                        out of 5 — from your trust network
                    </span>
                    <div style={{ fontSize: 12, color: "#888", marginTop: 4 }}>
                        {result.contributingReviews} of {result.totalReviewsConsidered} reviews contributed
                        (±{result.confidenceRange.toFixed(2)})
                    </div>
                </>
            )}
        </div>
    );
}
