import { useQuidnug } from "../provider.js";
import { useAsync } from "./useAsync.js";

/** Fetch the guardian set for a quid (QDP-0002). Returns null on 404. */
export function useGuardianSet(quidId) {
    const { client } = useQuidnug();
    return useAsync(
        () => client.getGuardianSet(quidId),
        [quidId],
        { enabled: !!quidId }
    );
}
