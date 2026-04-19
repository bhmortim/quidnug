import { useCallback, useState } from "react";
import { useQuidnug } from "@quidnug/react";

/**
 * Hook: post a new review. Returns { mutate, loading, error, data }.
 *
 *   const { mutate: postReview, loading } = useWriteReview();
 *   await postReview({
 *     product, topic, rating: 4.5, title, body,
 *   });
 */
export function useWriteReview() {
    const { client, quid } = useQuidnug();
    const [state, setState] = useState({ data: null, loading: false, error: null });

    const mutate = useCallback(async ({
        product, topic, rating, maxRating = 5.0, title, body, contextTags, locale,
    }) => {
        if (!client || !quid?.has_private_key) {
            throw new Error("Need a signed-in Quid to post reviews");
        }
        setState({ data: null, loading: true, error: null });
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
                        locale: locale ?? navigator.language ?? "en",
                        contextTags,
                    },
                },
                quid,
            );
            setState({ data: receipt, loading: false, error: null });
            return receipt;
        } catch (err) {
            setState({ data: null, loading: false, error: err });
            throw err;
        }
    }, [client, quid]);

    return { mutate, ...state };
}
