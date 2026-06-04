/**
 * Server-side helper used by Astro pages to fetch + compute the
 * trust-weighted rating for a (productId, observerId, topic) tuple.
 *
 * Wraps @quidnug/client + @quidnug/web-components/rater.js so an
 * Astro `--- frontmatter ---` block can resolve a fully-rendered
 * rating in one call and feed it straight into <QnAurora> /
 * <QnTrace> at build time. The SSR path therefore produces the
 * exact same SVG as the client-rendered custom element.
 *
 *   ---
 *   import { QnAurora, QnTrace } from "@quidnug/astro-reviews";
 *   import { computePersonalRating }
 *       from "@quidnug/astro-reviews/lib/rating.js";
 *
 *   const rating = await computePersonalRating({
 *       node: "http://localhost:8080",
 *       productId: Astro.props.productId,
 *       observerId: Astro.locals.user?.quidId,
 *       topic: "reviews.public.technology.laptops",
 *   });
 *   ---
 *
 *   <QnAurora rating={rating.rating}
 *             contributors={rating.contributingReviews}
 *             contributorRatings={rating.contributorRatings} />
 *   <QnTrace contributors={rating.contributors} showLabels />
 */
import QuidnugClient from "@quidnug/client";
import "@quidnug/client/v2";
import { Rater } from "@quidnug/web-components/rater.js";

/**
 * @param {Object}  opts
 * @param {string}  opts.node        Base URL of the Quidnug node (required).
 * @param {string}  opts.productId   Subject (title) id (required).
 * @param {string}  opts.topic       Topic domain (required).
 * @param {string} [opts.observerId] Viewer quid id. Falls back to "anonymous".
 * @param {string} [opts.productName] Optional display name passed through to result.
 * @param {Object} [opts.raterOptions] Forwarded to Rater (recencyHalflifeDays etc.).
 *
 * @returns {Promise<{
 *   productId: string, productName: string|undefined,
 *   observerId: string, topic: string,
 *   personal: number|null, crowd: number|null,
 *   rating: number|null,
 *   contributors: Array<{ id: string, name: string|undefined,
 *                          rating: number, weight: number, direct: boolean }>,
 *   contributorRatings: number[],
 *   contributingReviews: number, totalReviewsConsidered: number,
 *   confidenceRange: number,
 * }>}
 */
export async function computePersonalRating({
    node,
    productId,
    topic,
    observerId,
    productName,
    raterOptions,
    client: providedClient,
} = {}) {
    if (!productId) throw new Error("computePersonalRating: productId is required");
    if (!topic) throw new Error("computePersonalRating: topic is required");

    const client = providedClient ?? new QuidnugClient({ defaultNode: node });
    const observer = observerId || "anonymous";

    const rater = new Rater(client, raterOptions ?? {});
    const personal = await rater.effectiveRating(observer, productId, topic);

    // "Crowd" view: same algorithm run from an anonymous observer.
    // The Rater treats anonymous as "no trust edges" -> pure activity/recency.
    const crowd = observer === "anonymous"
        ? personal
        : await rater.effectiveRating("anonymous", productId, topic);

    const contributors = (personal.contributions ?? []).map((c) => ({
        id: c.reviewerQuid,
        name: c.reviewerName ?? c.reviewerQuid?.slice(0, 8),
        rating: c.rating,
        weight: c.weight,
        direct: !!c.direct,
    }));

    return {
        productId,
        productName,
        observerId: observer,
        topic,
        personal: personal.rating,
        crowd: crowd?.rating ?? null,
        rating: personal.rating,
        contributors,
        contributorRatings: contributors.map((c) => c.rating),
        contributingReviews: personal.contributingReviews ?? 0,
        totalReviewsConsidered: personal.totalReviewsConsidered ?? 0,
        confidenceRange: personal.confidenceRange ?? 0,
    };
}
