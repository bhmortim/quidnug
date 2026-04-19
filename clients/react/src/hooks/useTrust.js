import { useQuidnug } from "../provider.js";
import { useAsync } from "./useAsync.js";

/**
 * Query relational trust from `observer` to `target` in `domain`.
 *
 * Returns { data, error, loading, refetch } where data is:
 *   { trustLevel, trustPath, pathDepth }
 *
 * Any of the inputs may be null/undefined — the hook will sit idle
 * until all three are present, at which point it fetches once and
 * re-fetches on input change.
 */
export function useTrust(observer, target, domain, { maxDepth = 5 } = {}) {
    const { client, defaultDomain } = useQuidnug();
    const d = domain ?? defaultDomain;
    return useAsync(
        () => client.getTrustLevel(observer, target, d, { maxDepth }),
        [observer, target, d, maxDepth],
        { enabled: !!observer && !!target && !!d }
    );
}
