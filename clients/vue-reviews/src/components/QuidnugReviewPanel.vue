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

const { data, loading, error, refetch } = useTrustWeightedRating(
    () => props.product,
    () => props.topic,
);

const rating = computed(() => data.value?.rating ?? null);
</script>

<template>
    <div style="border:1px solid #e0e0e0;border-radius:8px;padding:16px;font-family:system-ui,sans-serif;color:#222">
        <div v-if="loading && !data" style="color:#888">
            Computing your weighted rating…
        </div>
        <div v-else-if="error" style="color:#c0392b">
            Rating unavailable: {{ error.message || String(error) }}
        </div>
        <div v-else>
            <h3 style="margin:0 0 4px 0;font-size:13px;color:#666;font-weight:500">
                Trust-weighted rating (your view)
            </h3>
            <em v-if="rating == null" style="color:#888">
                not enough trusted reviews for you yet
            </em>
            <template v-else>
                <span style="font-size:36px;font-weight:700;color:#0B1D3A">
                    {{ rating.toFixed(1) }}
                </span>
                <span style="font-size:14px;color:#666;margin-left:8px">
                    out of 5 — from your trust network
                </span>
                <div style="font-size:12px;color:#888;margin-top:4px">
                    {{ data.contributingReviews }} of {{ data.totalReviewsConsidered }} reviews contributed
                    (±{{ data.confidenceRange.toFixed(2) }})
                </div>
            </template>
        </div>

        <QuidnugReviewList
            :product="product"
            :topic="topic"
            :contributions="data?.contributions" />

        <QuidnugWriteReview v-if="showWrite"
            :product="product"
            :topic="topic"
            @success="refetch" />
    </div>
</template>
