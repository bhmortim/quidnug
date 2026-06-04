import { ref, watch } from "vue";
import { Rater } from "@quidnug/web-components/rater.js";
import { useQuidnug } from "../provider.js";

/**
 * Composable: per-observer trust-weighted rating for (product, topic).
 *
 * Mirrors @quidnug/react-reviews' useTrustWeightedRating().
 *
 *   const { data, loading, error, refetch } = useTrustWeightedRating(
 *       product, topic, { recencyHalflifeDays: 365 });
 *
 * `product`, `topic` may be plain strings, refs, or getters. The
 * composable re-runs whenever any reactive dep — including the
 * active quid in the provided context — changes.
 *
 * The returned refs are reactive; bind them directly in templates.
 */
export function useTrustWeightedRating(product, topic, options = {}) {
    const { client, quid } = useQuidnug();
    const data = ref(null);
    const loading = ref(false);
    const error = ref(null);
    let requestId = 0;

    const _unwrap = (v) => (typeof v === "function" ? v() : (v && "value" in v ? v.value : v));

    const refetch = async () => {
        const p = _unwrap(product);
        const t = _unwrap(topic);
        if (!p || !t || !client) {
            data.value = null;
            loading.value = false;
            error.value = null;
            return;
        }
        const id = ++requestId;
        loading.value = true;
        error.value = null;
        try {
            const rater = new Rater(client, options);
            const observerId = quid.value?.id ?? "anonymous";
            const result = await rater.effectiveRating(observerId, p, t);
            if (id === requestId) {
                data.value = result;
                loading.value = false;
            }
        } catch (err) {
            if (id === requestId) {
                data.value = null;
                loading.value = false;
                error.value = err;
            }
        }
    };

    watch(
        () => [_unwrap(product), _unwrap(topic), quid.value?.id],
        () => { refetch(); },
        { immediate: true }
    );

    return { data, loading, error, refetch };
}
