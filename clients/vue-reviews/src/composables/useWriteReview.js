import { ref, unref } from "vue";
import { useQuidnug } from "../context.js";

/**
 * Composable: post a new review. Returns { mutate, loading, error, data }.
 *
 *   const { mutate: postReview, loading } = useWriteReview();
 *   await postReview({
 *     product, topic, rating: 4.5, title, body,
 *   });
 */
export function useWriteReview() {
    const { client, quid } = useQuidnug();
    const data = ref(null);
    const loading = ref(false);
    const error = ref(null);

    const mutate = async ({
        product, topic, rating, maxRating = 5.0, title, body, contextTags, locale,
    }) => {
        const c = unref(client);
        const q = unref(quid);
        if (!c || !q?.has_private_key) {
            throw new Error("Need a signed-in Quid to post reviews");
        }
        data.value = null;
        loading.value = true;
        error.value = null;
        try {
            const receipt = await c.createEventTransaction(
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
                        locale: locale ?? (typeof navigator !== "undefined" ? navigator.language : null) ?? "en",
                        contextTags,
                    },
                },
                q,
            );
            data.value = receipt;
            loading.value = false;
            error.value = null;
            return receipt;
        } catch (err) {
            data.value = null;
            loading.value = false;
            error.value = err;
            throw err;
        }
    };

    return { mutate, data, loading, error };
}
