import React, { useState } from "react";
import { useQuidnug } from "@quidnug/react";
import { useWriteReview } from "../hooks/useWriteReview.js";

/**
 * <QuidnugWriteReview product topic onSuccess />
 *
 * Inline review-writing form. Requires a signed-in Quid.
 */
export function QuidnugWriteReview({ product, topic, onSuccess }) {
    const { quid } = useQuidnug();
    const { mutate, loading, error } = useWriteReview();
    const [rating, setRating] = useState(5);
    const [title, setTitle] = useState("");
    const [body, setBody] = useState("");

    if (!quid?.has_private_key) {
        return <p style={{ color: "#666", fontStyle: "italic", marginTop: 16 }}>
            Sign in with your Quidnug identity to write a review.
        </p>;
    }

    const submit = async (e) => {
        e.preventDefault();
        try {
            await mutate({ product, topic, rating, title, body });
            setTitle("");
            setBody("");
            setRating(5);
            onSuccess?.();
        } catch { /* shown via error state */ }
    };

    return (
        <form onSubmit={submit} style={{
            marginTop: 16, padding: 12, border: "1px solid #ddd",
            borderRadius: 8, background: "#fafafa",
        }}>
            <h4 style={{ margin: "0 0 8px", fontSize: 14 }}>Write a review</h4>

            <div style={{ marginBottom: 8, display: "flex", gap: 8, alignItems: "center" }}>
                <label style={{ fontSize: 13 }}>Your rating:</label>
                <input type="number" min={0} max={5} step={0.5}
                       value={rating} onChange={(e) => setRating(Number(e.target.value))}
                       style={{ width: 60, padding: 4 }} />
            </div>

            <input type="text" placeholder="Review title"
                   value={title} onChange={(e) => setTitle(e.target.value)}
                   style={{ width: "100%", padding: 8, marginBottom: 8,
                            border: "1px solid #ccc", borderRadius: 4,
                            boxSizing: "border-box" }} />

            <textarea placeholder="Your experience..."
                      value={body} onChange={(e) => setBody(e.target.value)}
                      rows={5}
                      style={{ width: "100%", padding: 8,
                               border: "1px solid #ccc", borderRadius: 4,
                               boxSizing: "border-box",
                               fontFamily: "inherit", fontSize: 13 }} />

            {error && <div style={{ color: "#c0392b", fontSize: 12, margin: "8px 0" }}>
                {String(error.message ?? error)}
            </div>}

            <button type="submit" disabled={loading}
                    style={{ marginTop: 8, padding: "8px 16px", background: "#2E8B57",
                             color: "#fff", border: "none", borderRadius: 4,
                             cursor: loading ? "not-allowed" : "pointer",
                             opacity: loading ? 0.6 : 1 }}>
                {loading ? "Submitting…" : "Post review"}
            </button>

            <p style={{ color: "#777", fontSize: 11, marginTop: 8 }}>
                Signed with your Quidnug identity. Published to <code>reviews.public.*</code> —
                your reputation carries across every site that speaks QRP-0001.
            </p>
        </form>
    );
}
