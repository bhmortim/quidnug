import { ref, watch, onBeforeUnmount, toRef, isRef, unref } from "vue";
import { Rater } from "@quidnug/web-components/rater.js";
import { useQuidnug } from "../context.js";

/**
 * Composable: per-observer trust-weighted rating for (product, topic).
 *
 * Mirrors the React `useTrustWeightedRating` hook one-for-one.
 *
 * Returns refs: { data, loading, error, refetch }.
 *
 * `product` and `topic` may be plain strings or refs. Passing refs
 * makes the composable re-fetch automatically when they change.
 *
 * `data` matches the WeightedRatingResult shape — see
 * examples/reviews-and-comments/algorithm.py.
 */
export function useTrustWeightedRating(product, topic, options = {}) {
    const { client, quid } = useQuidnug();
    const data = ref(null);
    const loading = ref(false);
    const error = ref(null);

    const productRef = isRef(product) ? product : toRef(() => product);
    const topicRef = isRef(topic) ? topic : toRef(() => topic);

    let mounted = true;
    let requestId = 0;
    onBeforeUnmount(() => { mounted = false; });

    const run = async () => {
        const p = unref(productRef);
        const t = unref(topicRef);
        const c = unref(client);
        if (!p || !t || !c) {
            data.value = null;
            loading.value = false;
            error.value = null;
            return;
        }
        const id = ++requestId;
        loading.value = true;
        error.value = null;
        try {
            const rater = new Rater(c, options);
            const observerQuid = unref(quid);
            const observer = observerQuid?.id ?? "anonymous";
            const result = await rater.effectiveRating(observer, p, t);
            if (id === requestId && mounted) {
                data.value = result;
                loading.value = false;
                error.value = null;
            }
        } catch (err) {
            if (id === requestId && mounted) {
                data.value = null;
                loading.value = false;
                error.value = err;
            }
        }
    };

    watch(
        [productRef, topicRef, client, () => unref(quid)?.id],
        () => { run(); },
        { immediate: true },
    );

    return { data, loading, error, refetch: run };
}
