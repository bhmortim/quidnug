<script setup>
import { computed, watch } from "vue";
import { useTrustWeightedRating } from "../composables/useTrustWeightedRating.js";

const props = defineProps({
    product: { type: String, required: true },
    topic: { type: String, required: true },
    max: { type: Number, default: 5 },
    showCount: { type: Boolean, default: false },
});
const emit = defineEmits(["rating"]);

const { data, loading, error } = useTrustWeightedRating(
    () => props.product,
    () => props.topic,
);

watch(data, (v) => { if (v) emit("rating", v); });

const rating = computed(() => data.value?.rating ?? null);

function fillFor(i) {
    const r = rating.value;
    if (r == null) return 0;
    if (r >= i) return 1;
    if (r >= i - 0.5) return 0.5;
    return 0;
}
const stars = computed(() => Array.from({ length: props.max }, (_, i) => fillFor(i + 1)));
const color = "#F9A825";
const empty = "#e0e0e0";
</script>

<template>
    <span v-if="loading" class="qn-stars-loading">…</span>
    <span v-else-if="error" class="qn-stars-error" style="color:#c0392b">⚠</span>
    <span v-else-if="rating == null"
          class="qn-stars-empty"
          style="color:#888;font-style:italic">not enough trusted reviews</span>
    <span v-else class="qn-stars"
          style="font-family:system-ui"
          :title="`${data.contributingReviews} of ${data.totalReviewsConsidered} contributed`">
        <span style="display:inline-flex;gap:2px">
            <svg v-for="(fill, i) in stars" :key="i"
                 width="18" height="18" viewBox="0 0 24 24">
                <defs v-if="fill === 0.5">
                    <linearGradient :id="`qng-half-${i}`">
                        <stop offset="50%" :stop-color="color" />
                        <stop offset="50%" :stop-color="empty" />
                    </linearGradient>
                </defs>
                <path :fill="fill === 1 ? color : fill === 0.5 ? `url(#qng-half-${i})` : empty"
                      d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
            </svg>
        </span>
        <span style="margin-left:6px;font-weight:600">{{ rating.toFixed(1) }}</span>
        <span style="margin-left:4px;font-size:12px;color:#666">
            ± {{ data.confidenceRange.toFixed(2) }}
        </span>
        <span v-if="showCount" style="margin-left:4px;font-size:12px;color:#666">
            ({{ data.contributingReviews }} trusted)
        </span>
    </span>
</template>
