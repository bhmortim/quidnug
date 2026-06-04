import { ref, watch } from "vue";
import { useQuidnug } from "../provider.js";

/**
 * Composable: fetch REVIEW events on a product within a topic domain.
 *
 * Mirrors @quidnug/react-reviews' useReviews().
 *
 *   const { data, loading, error, refetch } = useReviews(product, topic, {
 *       limit: 50, offset: 0,
 *   });
 *
 * `data` is a ref<Event[]> filtered to eventType === "REVIEW".
 * `product` and `topic` may be plain strings, refs, or getters.
 */
export function useReviews(product, topic, { limit = 50, offset = 0 } = {}) {
    const { client } = useQuidnug();
    const data = ref([]);
    const loading = ref(false);
    const error = ref(null);
    let requestId = 0;

    const _unwrap = (v) => (typeof v === "function" ? v() : (v && "value" in v ? v.value : v));

    const refetch = async () => {
        const p = _unwrap(product);
        const t = _unwrap(topic);
        if (!p || !t || !client) return;
        const id = ++requestId;
        loading.value = true;
        error.value = null;
        try {
            const res = await client.getStreamEvents(p, { domain: t, limit, offset });
            const reviews = (res.events ?? []).filter((e) => e.eventType === "REVIEW");
            if (id === requestId) {
                data.value = reviews;
                loading.value = false;
            }
        } catch (err) {
            if (id === requestId) {
                data.value = [];
                loading.value = false;
                error.value = err;
            }
        }
    };

    watch(
        () => [_unwrap(product), _unwrap(topic), limit, offset],
        () => { refetch(); },
        { immediate: true }
    );

    return { data, loading, error, refetch };
}
