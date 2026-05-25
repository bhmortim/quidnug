<!--
 * <QuidnugReviewPanel product topic :show-write />
 *
 * Full review experience for a product page: headline trust-weighted
 * rating at top, review list below, inline write form at the bottom
 * (if show-write is set).
-->
<script setup>
import { computed } from "vue";
import { useTrustWeightedRating } from "../composables/useTrustWeightedRating.js";
import QuidnugReviewList from "./QuidnugReviewList.vue";
import QuidnugWriteReview from "./QuidnugWriteReview.vue";

const props = defineProps({
    product: { type: String, required: true },
    topic: { type: String, required: true },
    showWrite: { type: Boolean, default: false },
});

const rating = useTrustWeightedRating(
    computed(() => props.product),
    computed(() => props.topic),
);

const result = computed(() => rating.data.value);
const ratingValue = computed(() => result.value?.rating ?? null);
const contributions = computed(() => result.value?.contributions ?? null);
</script>

<template>
    <div :style="{
        border: '1px solid #e0e0e0',
        borderRadius: '8px',
        padding: '16px',
        fontFamily: 'system-ui, sans-serif',
        color: '#222',
    }">
        <div v-if="rating.loading.value && !result"
             :style="{ color: '#888' }">
            Computing your weighted rating…
        </div>
        <div v-else-if="rating.error.value"
             :style="{ color: '#c0392b' }">
            Rating unavailable: {{ String(rating.error.value.message ?? rating.error.value) }}
        </div>
        <div v-else>
            <h3 :style="{ margin: '0 0 4px 0', fontSize: '13px', color: '#666', fontWeight: 500 }">
                Trust-weighted rating (your view)
            </h3>
            <em v-if="ratingValue == null" :style="{ color: '#888' }">
                not enough trusted reviews for you yet
            </em>
            <template v-else>
                <span :style="{ fontSize: '36px', fontWeight: 700, color: '#0B1D3A' }">
                    {{ ratingValue.toFixed(1) }}
                </span>
                <span :style="{ fontSize: '14px', color: '#666', marginLeft: '8px' }">
                    out of 5 — from your trust network
                </span>
                <div :style="{ fontSize: '12px', color: '#888', marginTop: '4px' }">
                    {{ result.contributingReviews }} of {{ result.totalReviewsConsidered }} reviews contributed
                    (±{{ result.confidenceRange.toFixed(2) }})
                </div>
            </template>
        </div>

        <QuidnugReviewList
            :product="product"
            :topic="topic"
            :contributions="contributions" />

        <QuidnugWriteReview
            v-if="showWrite"
            :product="product"
            :topic="topic"
            @success="rating.refetch" />
    </div>
</template>
