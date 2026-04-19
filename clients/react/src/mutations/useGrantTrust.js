import { useQuidnug } from "../provider.js";
import { useMutation } from "../hooks/useAsync.js";

/**
 * Returns { mutate, data, error, pending } where mutate is:
 *   ({ trustee, level, domain?, nonce?, validUntil?, description?, quid? }) => Promise
 */
export function useGrantTrust() {
    const { client, quid: activeQuid, defaultDomain } = useQuidnug();

    return useMutation(async ({
        trustee, level, domain, nonce, validUntil, description, quid,
    }) => {
        const signer = quid ?? activeQuid;
        if (!signer) throw new Error("no active quid");
        const tx = await client.createTrustTransaction({
            trustee,
            trustLevel: level,
            domain: domain ?? defaultDomain,
            nonce,
            validUntil,
            description,
        }, signer);
        return client.submitTransaction(tx);
    });
}
