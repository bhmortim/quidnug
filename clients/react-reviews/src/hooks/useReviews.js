import { useEffect, useRef, useState } from "react";
import { useQuidnug } from "@quidnug/react";

/**
 * Hook: fetch reviews on a product in a topic domain.
 *
 * Returns { data: Event[], loading, error, refetch }
 */
export function useReviews(product, topic, { limit = 50, offset = 0 } = {}) {
    const { client } = useQuidnug();
    const [state, setState] = useState({ data: [], loading: false, error: null });
    const mounted = useRef(true);
    const requestId = useRef(0);

    useEffect(() => () => { mounted.current = false; }, []);

    const run = async () => {
        if (!product || !topic || !client) return;
        const id = ++requestId.current;
        setState((s) => ({ ...s, loading: true, error: null }));
        try {
            const res = await client.getStreamEvents(product, {
                domain: topic,
                limit,
                offset,
            });
            const reviews = (res.events ?? []).filter((e) => e.eventType === "REVIEW");
            if (id === requestId.current && mounted.current) {
                setState({ data: reviews, loading: false, error: null });
            }
        } catch (err) {
            if (id === requestId.current && mounted.current) {
                setState({ data: [], loading: false, error: err });
            }
        }
    };

    useEffect(() => {
        run();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [product, topic, limit, offset]);

    return { ...state, refetch: run };
}
