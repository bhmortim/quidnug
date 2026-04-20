<!--
 * <QnTrace /> — Vue 3 wrapper around <qn-trace>.
-->
<script setup>
import { computed } from "vue";
import "@quidnug/web-components/src/primitives/index.js";

const props = defineProps({
    contributors: { type: Array, default: () => [] },
    max: { type: Number, default: 5 },
    height: { type: Number, default: 10 },
    showLabels: { type: Boolean, default: false },
    emptyText: { type: String, default: "" },
});

const contributorsJson = computed(() => JSON.stringify(props.contributors));
</script>

<template>
    <qn-trace
        :contributors="contributorsJson"
        :max="String(max)"
        :height="String(height)"
        :show-labels="showLabels ? '' : null"
        :empty-text="emptyText || null"></qn-trace>
</template>
