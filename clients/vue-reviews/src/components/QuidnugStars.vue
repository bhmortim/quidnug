<!--
 * <QuidnugStars product topic />
 *
 * Drop-in trust-weighted star rating for Vue apps. Requires
 * `provideQuidnug({ client, quid })` higher in the tree.
 *
 * Props mirror the React component one-for-one.
-->
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
    computed(() => props.product),
    computed(() => props.topic),
);

watch(data, (v) => { if (v) emit("rating", v); });

const rating = computed(() => data.value?.rating ?? null);

function starFill(i) {
    const v = rating.value;
    if (v == null) return 0;
    if (v >= i) return 1;
    if (v >= i - 0.5) return 0.5;
    return 0;
}

const stars = computed(() => {
    const out = [];
    for (let i = 1; i <= props.max; i++) out.push({ i, fill: starFill(i) });
    return out;
});

function gradId(i) {
    return `qng-half-${i}-${Math.random().toString(36).slice(2)}`;
}
</script>

<template>
    <span v-if="loading">…</span>
    <span v-else-if="error" :style="{ color: '#c0392b' }">⚠</span>
    <span v-else-if="rating == null"
          :style="{ color: '#888', fontStyle: 'italic' }">
        not enough trusted reviews
    </span>
    <span v-else
          :style="{ fontFamily: 'system-ui' }"
          :title="`${data.contributingReviews} of ${data.totalReviewsConsidered} contributed`">
        <span :style="{ display: 'inline-flex', gap: '2px' }">
            <svg v-for="s in stars" :key="s.i" width="18" height="18" viewBox="0 0 24 24">
                <template v-if="s.fill === 0">
                    <path fill="#e0e0e0"
                          d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
                </template>
                <template v-else-if="s.fill === 1">
                    <path fill="#F9A825"
                          d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
                </template>
                <template v-else>
                    <defs>
                        <linearGradient :id="gradId(s.i)">
                            <stop offset="50%" stop-color="#F9A825" />
                            <stop offset="50%" stop-color="#e0e0e0" />
                        </linearGradient>
                    </defs>
                    <path :fill="`url(#${gradId(s.i)})`"
                          d="M12 2l3 7 7 .6-5.3 4.7 1.7 7.2L12 17.8 5.6 21.5l1.7-7.2L2 9.6 9 9z" />
                </template>
            </svg>
        </span>
        <span :style="{ marginLeft: '6px', fontWeight: 600 }">{{ rating.toFixed(1) }}</span>
        <span :style="{ marginLeft: '4px', fontSize: '12px', color: '#666' }">
            ± {{ data.confidenceRange.toFixed(2) }}
        </span>
        <span v-if="showCount" :style="{ marginLeft: '4px', fontSize: '12px', color: '#666' }">
            ({{ data.contributingReviews }} trusted)
        </span>
    </span>
</template>
