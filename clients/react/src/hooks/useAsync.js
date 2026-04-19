import { useEffect, useRef, useState, useCallback } from "react";

/**
 * Shared async-data primitive used by every read hook.
 *
 * Returns:
 *   { data, error, loading, refetch }
 *
 * - `data` is null until the first successful load.
 * - `error` is the caught Error (or null).
 * - `loading` is true during every fetch including refetches.
 * - `refetch()` re-runs the fetcher ignoring cache.
 *
 * Cancels in-flight requests when dependencies change or the
 * component unmounts, avoiding "setState on unmounted component"
 * warnings.
 */
export function useAsync(fetcher, deps, { enabled = true } = {}) {
    const [data, setData] = useState(null);
    const [error, setError] = useState(null);
    const [loading, setLoading] = useState(false);
    const requestId = useRef(0);
    const mounted = useRef(true);

    useEffect(() => () => { mounted.current = false; }, []);

    const run = useCallback(() => {
        const id = ++requestId.current;
        setLoading(true);
        setError(null);
        Promise.resolve()
            .then(fetcher)
            .then((val) => {
                if (id === requestId.current && mounted.current) {
                    setData(val);
                    setLoading(false);
                }
            })
            .catch((err) => {
                if (id === requestId.current && mounted.current) {
                    setError(err);
                    setLoading(false);
                }
            });
    }, deps); // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
        if (enabled) run();
    }, [run, enabled]);

    return { data, error, loading, refetch: run };
}

/** Shared async-mutation primitive used by every write hook. */
export function useMutation(fn) {
    const [state, setState] = useState({ data: null, error: null, pending: false });
    const mounted = useRef(true);
    useEffect(() => () => { mounted.current = false; }, []);

    const mutate = useCallback(async (...args) => {
        setState((s) => ({ ...s, pending: true, error: null }));
        try {
            const res = await fn(...args);
            if (mounted.current) {
                setState({ data: res, error: null, pending: false });
            }
            return res;
        } catch (err) {
            if (mounted.current) {
                setState({ data: null, error: err, pending: false });
            }
            throw err;
        }
    }, [fn]);

    return { ...state, mutate };
}
