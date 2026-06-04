import { ref } from "vue";
import { useQuidnug } from "../provider.js";

/**
 * Composable: post a new REVIEW event for a product.
 *
 * Mirrors @quidnug/react-reviews' useWriteReview().
 *
 *   const { mutate, loading, error, data } = useWriteReview();
 *   await mutate({ product, topic, rating: 4.5, title, body });
 *
 * `mutate` resolves with the server's transaction receipt.
 * Requires a signed-in quid with private key in the provided context.
 */
export function useWriteReview() {
    const { client, quid } = useQuidnug();
    const data = ref(null);
    const loading = ref(false);
    const error = ref(null);

    const mutate = async ({
        product, topic, rating, maxRating = 5.0,
        title, body, contextTags, locale,
    }) => {
        if (!client || !quid.value?.has_private_key) {
            throw new Error("Need a signed-in Quid (with private key) to post reviews");
        }
        loading.value = true;
        error.value = null;
        try {
            const receipt = await client.createEventTransaction(
                {
                    subjectId: product,
                    subjectType: "TITLE",
                    eventType: "REVIEW",
                    domain: topic,
                    payload: {
                        qrpVersion: 1,
                        rating,
                        maxRating,
                        title,
                        bodyMarkdown: body,
                        locale: locale ?? (typeof navigator !== "undefined" ? navigator.language : "en"),
                        contextTags,
                    },
                },
                quid.value,
            );
            data.value = receipt;
            loading.value = false;
            return receipt;
        } catch (err) {
            error.value = err;
            loading.value = false;
            throw err;
        }
    };

    return { mutate, data, loading, error };
}
