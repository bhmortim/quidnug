<!--
 * <QnAurora /> — Vue 3 wrapper around the <qn-aurora> custom
 * element. Props mirror the underlying attributes one-for-one.
 *
 * Events:
 *   @aurora-click  — fires with the aurora detail object
 *
 * Usage:
 *   <QnAurora :rating="4.5" :contributors="7" :direct="5"
 *             :crowd="4.1" observer-name="alice"
 *             show-delta show-histogram
 *             :contributor-ratings="[4.5, 4.8, 4.2, 5, 4, 4.3, 4.7]"
 *             @aurora-click="openDrilldown" />
-->
<script setup>
import { onMounted, onBeforeUnmount, ref, computed, watch } from "vue";
import "@quidnug/web-components/src/primitives/index.js";

const props = defineProps({
    rating: { type: Number, default: null },
    max: { type: Number, default: 5 },
    crowd: { type: Number, default: null },
    contributors: { type: Number, default: 0 },
    direct: { type: Number, default: 0 },
    size: { type: String, default: "standard" },
    observerName: { type: String, default: "" },
    showValue: { type: Boolean, default: false },
    showDelta: { type: Boolean, default: false },
    showHistogram: { type: Boolean, default: false },
    contributorRatings: { type: Array, default: null },
});
const emit = defineEmits(["aurora-click"]);
const el = ref(null);

function handle(e) { emit("aurora-click", e.detail); }
onMounted(() => { el.value?.addEventListener("qn-aurora-click", handle); });
onBeforeUnmount(() => { el.value?.removeEventListener("qn-aurora-click", handle); });

const ratingsJson = computed(() =>
    props.contributorRatings ? JSON.stringify(props.contributorRatings) : null);
</script>

<template>
    <qn-aurora ref="el"
        :rating="rating != null ? String(rating) : null"
        :max="String(max)"
        :crowd="crowd != null ? String(crowd) : null"
        :contributors="String(contributors)"
        :direct="String(direct)"
        :size="size"
        :observer-name="observerName || null"
        :show-value="showValue ? '' : null"
        :show-delta="showDelta ? '' : null"
        :show-histogram="showHistogram ? '' : null"
        :contributor-ratings="ratingsJson"></qn-aurora>
</template>
