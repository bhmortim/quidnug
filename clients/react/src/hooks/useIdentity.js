import { useQuidnug } from "../provider.js";
import { useAsync } from "./useAsync.js";

/** Fetch the identity record for a quid. Returns null on 404. */
export function useIdentity(quidId, { domain } = {}) {
    const { client, defaultDomain } = useQuidnug();
    const d = domain ?? defaultDomain;
    return useAsync(
        () => client.getIdentity(quidId, d),
        [quidId, d],
        { enabled: !!quidId }
    );
}
