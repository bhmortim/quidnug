import { useEffect, useRef, useState } from "react";
import { useQuidnug } from "@quidnug/react";
import { Rater } from "@quidnug/web-components/rater.js";

/**
 * Hook: per-observer trust-weighted rating for (product, topic).
 *
 * Returns { data, loading, error, refetch }
 *
 * `data` shape: see examples/reviews-and-comments/algorithm.py
 * `WeightedRatingResult`.
 */
export function useTrustWeightedRating(product, topic, options = {}) {
    const { client, quid } = useQuidnug();
    const [state, setState] = useState({ data: null, loading: false, error: null });
    const mounted = useRef(true);
    const requestId = useRef(0);

    useEffect(() => () => { mounted.current = false; }, []);

    const run = async () => {
        if (!product || !topic || !client) {
            setState({ data: null, loading: false, error: null });
            return;
        }
        const id = ++requestId.current;
        setState((s) => ({ ...s, loading: true, error: null }));
        try {
            const rater = new Rater(client, options);
            const observer = quid?.id ?? "anonymous";
            const result = await rater.effectiveRating(observer, product, topic);
            if (id === requestId.current && mounted.current) {
                setState({ data: result, loading: false, error: null });
            }
        } catch (err) {
            if (id === requestId.current && mounted.current) {
                setState({ data: null, loading: false, error: err });
            }
        }
    };

    useEffect(() => {
        run();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [product, topic, client, quid?.id]);

    return { ...state, refetch: run };
}
