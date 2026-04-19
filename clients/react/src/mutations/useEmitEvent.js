import { useQuidnug } from "../provider.js";
import { useMutation } from "../hooks/useAsync.js";

/**
 * Returns { mutate, data, error, pending } where mutate is:
 *   ({ subjectId, subjectType, eventType, payload|payloadCID, domain?, sequence?, quid? }) => Promise
 */
export function useEmitEvent() {
    const { client, quid: activeQuid, defaultDomain } = useQuidnug();

    return useMutation(async ({
        subjectId, subjectType, eventType, payload, payloadCID,
        domain, sequence, quid,
    }) => {
        const signer = quid ?? activeQuid;
        if (!signer) throw new Error("no active quid");
        return client.createEventTransaction({
            subjectId,
            subjectType,
            eventType,
            payload,
            payloadCID,
            domain: domain ?? defaultDomain,
            sequence,
        }, signer);
    });
}
