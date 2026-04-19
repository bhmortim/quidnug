import { useQuidnug } from "../provider.js";
import { useAsync } from "./useAsync.js";

/**
 * Fetch a paginated slice of `subjectId`'s event stream.
 *
 * Returns { data: { events, pagination }, error, loading, refetch }.
 *
 * For real-time updates, combine with a polling timer or a WebSocket
 * subscription — the hook itself does not auto-refresh.
 */
export function useStream(subjectId, { limit = 50, offset = 0, domain } = {}) {
    const { client, defaultDomain } = useQuidnug();
    const d = domain ?? defaultDomain;
    return useAsync(
        () => client.getStreamEvents(subjectId, { limit, offset, domain: d }),
        [subjectId, limit, offset, d],
        { enabled: !!subjectId }
    );
}
