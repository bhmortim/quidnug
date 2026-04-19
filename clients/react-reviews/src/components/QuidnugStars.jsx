import React from "react";
import { useTrustWeightedRating } from "../hooks/useTrustWeightedRating.js";

/**
 * <QuidnugStars product="..." topic="..." />
 *
 * Drop-in trust-weighted star rating for React apps. Uses
 * <QuidnugProvider> higher in the tree.
 */
export function QuidnugStars({
    product,
    topic,
    max = 5,
    showCount = false,
    onRating,
    style,
    className,
}) {
    const { data, loading, error } = useTrustWeightedRating(product, topic);

    React.useEffect(() => {
        if (onRating && data) onRating(data);
    }, [data, onRating]);

    if (loading) return <span className={className} style={style}>…</span>;
    if (error) return <span style={{ color: "#c0392b", ...style }} className={className}>⚠</span>;

    const rating = data?.rating;
    if (rating == null) {
        return (
            <span className={className} style={{ color: "#888", fontStyle: "italic", ...style }}>
                not enough trusted reviews
            </span>
        );
    }

    return (
        <span className={className} style={{ fontFamily: "system-ui", ...style }}
              title={`${data.contributingReviews} of ${data.totalReviewsConsidered} contributed`}>
            <Stars value={rating} max={max} />
            <span style={{ marginLeft: 6, fontWeight: 600 }}>
                {rating.toFixed(1)}
            </span>
            <span style={{ marginLeft: 4, fontSize: 12, color: "#666" }}>
                ± {data.confidenceRange.toFixed(2)}
            </span>
            {showCount && (
                <span style={{ marginLeft: 4, fontSize: 12, color: "#666" }}>
                    ({data.contributingReviews} trusted)
                </span>
            )}
        </span>
    );
}

function Stars({ value, max }) {
    const items = [];
    for (let i = 1; i <= max; i++) {
        if (value >= i) {
            items.push(<Star key={i} fill={1} />);
        } else if (value >= i - 0.5) {
            items.push(<Star key={i} fill={0.5} />);
        } else {
            items.push(<Star key={i} fill={0} />);
        }
    }
    return <span style={{ display: "inline-flex", gap: 2 }}>{items}</span>;
}

function Star({ fill }) {
    const color = "#F9A825";
    const empty = "#e0e0e0";
    if (fill === 0) {
        return (
            <svg width="18" height="18" viewBox="0 0 24 24">
                <path fill={empty} d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
            </svg>
        );
    }
    if (fill === 1) {
        return (
            <svg width="18" height="18" viewBox="0 0 24 24">
                <path fill={color} d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
            </svg>
        );
    }
    // half
    const gradId = `qng-half-${Math.random().toString(36).slice(2)}`;
    return (
        <svg width="18" height="18" viewBox="0 0 24 24">
            <defs>
                <linearGradient id={gradId}>
                    <stop offset="50%" stopColor={color} />
                    <stop offset="50%" stopColor={empty} />
                </linearGradient>
            </defs>
            <path fill={`url(#${gradId})`}
                  d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
        </svg>
    );
}
