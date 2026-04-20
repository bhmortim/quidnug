<!--
 * <QnConstellation /> — Vue 3 wrapper around <qn-constellation>.
 *
 * Emits @select with the contributor object on dot click.
-->
<script setup>
import { onMounted, onBeforeUnmount, ref, computed } from "vue";
import "@quidnug/web-components/src/primitives/index.js";

const props = defineProps({
    contributors: { type: Array, default: () => [] },
    max: { type: Number, default: 5 },
    size: { type: String, default: "standard" },
    titleText: { type: String, default: "" },
    observerName: { type: String, default: "" },
});
const emit = defineEmits(["select"]);
const el = ref(null);

function handle(e) { emit("select", e.detail); }
onMounted(() => { el.value?.addEventListener("qn-constellation-select", handle); });
onBeforeUnmount(() => { el.value?.removeEventListener("qn-constellation-select", handle); });

const contributorsJson = computed(() => JSON.stringify(props.contributors));
</script>

<template>
    <qn-constellation ref="el"
        :contributors="contributorsJson"
        :max="String(max)"
        :size="size"
        :title-text="titleText || null"
        :observer-name="observerName || null"></qn-constellation>
</template>
