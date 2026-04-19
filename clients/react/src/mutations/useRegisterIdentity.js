import { useQuidnug } from "../provider.js";
import { useMutation } from "../hooks/useAsync.js";

/**
 * Returns { mutate, data, error, pending } where mutate is:
 *   ({ name?, homeDomain?, attributes?, quid? }) => Promise<receipt>
 *
 * If `quid` is omitted, the provider's active Quid is used.
 */
export function useRegisterIdentity() {
    const { client, quid: activeQuid, defaultDomain } = useQuidnug();

    return useMutation(async (params = {}) => {
        const signer = params.quid ?? activeQuid;
        if (!signer) throw new Error("no active quid; pass one or set via QuidnugProvider");
        const tx = await client.createIdentityTransaction({
            subjectQuid: signer.id,
            domain: params.domain ?? defaultDomain,
            name: params.name,
            description: params.description,
            attributes: params.attributes,
            homeDomain: params.homeDomain,
        }, signer);
        return client.submitTransaction(tx);
    });
}
