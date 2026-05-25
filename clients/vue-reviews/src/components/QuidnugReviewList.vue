<!--
 * <QuidnugReviewList product topic />
 *
 * Renders reviews sorted by your-trust-weight. Accepts an optional
 * pre-computed `contributions` array to avoid fetching twice when
 * used inside <QuidnugReviewPanel>.
-->
<script setup>
import { computed } from "vue";
import { useReviews } from "../composables/useReviews.js";

const props = defineProps({
    product: { type: String, required: true },
    topic: { type: String, required: true },
    contributions: { type: Array, default: null },
    sort: { type: String, default: "weight" },
    limit: { type: Number, default: 20 },
});

const { data: reviews, loading, error } = useReviews(
    computed(() => props.product),
    computed(() => props.topic),
    { limit: computed(() => props.limit) },
);

const weightMap = computed(() => {
    if (!props.contributions) return new Map();
    return new Map(props.contributions.map((c) => [c.reviewerQuid, c.weight]));
});

const sorted = computed(() => {
    const copy = [...(reviews.value ?? [])];
    const wm = weightMap.value;
    const byWeight = (a, b) => (wm.get(b.creator) ?? 0) - (wm.get(a.creator) ?? 0);
    const byRecent = (a, b) => (b.timestamp ?? 0) - (a.timestamp ?? 0);
    const byHigh = (a, b) => (b.payload?.rating ?? 0) - (a.payload?.rating ?? 0);
    const byLow = (a, b) => (a.payload?.rating ?? 0) - (b.payload?.rating ?? 0);
    switch (props.sort) {
        case "recent":      copy.sort(byRecent); break;
        case "rating-high": copy.sort(byHigh); break;
        case "rating-low":  copy.sort(byLow); break;
        case "weight":
        default:            copy.sort(byWeight);
    }
    return copy;
});

function weightOf(ev) { return weightMap.value.get(ev.creator); }
function contributing(ev) { const w = weightOf(ev); return w != null && w > 0.1; }
function creatorShort(ev) { return String(ev.creator ?? "").slice(0, 10); }
</script>

<template>
    <div v-if="loading" :style="{ color: '#888' }">Loading reviews…</div>
    <div v-else-if="error" :style="{ color: '#c0392b' }">{{ String(error.message ?? error) }}</div>
    <em v-else-if="sorted.length === 0" :style="{ color: '#888' }">No reviews yet</em>
    <div v-else :style="{ display: 'flex', flexDirection: 'column', gap: '12px', marginTop: '12px' }">
        <div v-for="ev in sorted"
             :key="`${ev.subjectId}-${ev.sequence}`"
             :style="{
                 padding: '10px 12px',
                 borderRadius: '6px',
                 background: '#f7f7f7',
                 borderLeft: `3px solid ${contributing(ev) ? '#2E8B57' : '#ccc'}`,
                 opacity: contributing(ev) ? 1 : 0.55,
                 fontSize: '14px',
             }">
            <div :style="{ display: 'flex', gap: '8px', alignItems: 'center' }">
                <span :style="{ color: '#F9A825', fontWeight: 600 }">★ {{ ev.payload?.rating }}</span>
                <strong v-if="ev.payload?.title">{{ ev.payload.title }}</strong>
                <span :style="{
                    fontSize: '11px',
                    color: contributing(ev) ? '#2E8B57' : '#999',
                    fontFamily: 'monospace',
                }">
                    {{ weightOf(ev) != null ? `weight ${weightOf(ev).toFixed(2)}` : "outside your trust" }}
                </span>
            </div>
            <div :style="{ fontFamily: 'monospace', fontSize: '11px', color: '#888', margin: '4px 0' }">
                by {{ creatorShort(ev) }}…
            </div>
            <div>{{ ev.payload?.bodyMarkdown }}</div>
        </div>
    </div>
</template>
