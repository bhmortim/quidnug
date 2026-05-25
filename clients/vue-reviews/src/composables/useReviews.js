import { ref, watch, onBeforeUnmount, toRef, isRef, unref } from "vue";
import { useQuidnug } from "../context.js";

/**
 * Composable: fetch reviews on a product in a topic domain.
 *
 * Mirrors the React `useReviews` hook.
 *
 * Returns refs: { data, loading, error, refetch }. `data` is an
 * Event[] of REVIEW events.
 */
export function useReviews(product, topic, opts = {}) {
    const { limit = 50, offset = 0 } = opts;
    const { client } = useQuidnug();

    const data = ref([]);
    const loading = ref(false);
    const error = ref(null);

    const productRef = isRef(product) ? product : toRef(() => product);
    const topicRef = isRef(topic) ? topic : toRef(() => topic);
    const limitRef = isRef(limit) ? limit : toRef(() => limit);
    const offsetRef = isRef(offset) ? offset : toRef(() => offset);

    let mounted = true;
    let requestId = 0;
    onBeforeUnmount(() => { mounted = false; });

    const run = async () => {
        const p = unref(productRef);
        const t = unref(topicRef);
        const c = unref(client);
        if (!p || !t || !c) return;
        const id = ++requestId;
        loading.value = true;
        error.value = null;
        try {
            const res = await c.getStreamEvents(p, {
                domain: t,
                limit: unref(limitRef),
                offset: unref(offsetRef),
            });
            const reviews = (res.events ?? []).filter((e) => e.eventType === "REVIEW");
            if (id === requestId && mounted) {
                data.value = reviews;
                loading.value = false;
                error.value = null;
            }
        } catch (err) {
            if (id === requestId && mounted) {
                data.value = [];
                loading.value = false;
                error.value = err;
            }
        }
    };

    watch(
        [productRef, topicRef, limitRef, offsetRef, client],
        () => { run(); },
        { immediate: true },
    );

    return { data, loading, error, refetch: run };
}
