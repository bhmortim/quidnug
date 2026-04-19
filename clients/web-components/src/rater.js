/**
 * Client-side port of the trust-weighted rating algorithm (matches
 * examples/reviews-and-comments/algorithm.py). Wraps the Quidnug
 * JS client v2 to fetch reviews, votes, and trust edges, then
 * computes per-observer effective ratings.
 *
 * Intentionally dependency-light — works with @quidnug/client v2
 * (which itself has no runtime deps).
 */

export class Rater {
    constructor(client, config = {}) {
        this.client = client;
        this.config = {
            recencyHalflifeDays: config.recencyHalflifeDays ?? 730,
            recencyFloor:        config.recencyFloor ?? 0.3,
            activitySaturation:  config.activitySaturation ?? 50,
            topicInheritanceDecay: config.topicInheritanceDecay ?? 0.8,
            minWeightThreshold:  config.minWeightThreshold ?? 0.01,
            maxDepthT:           config.maxDepthT ?? 5,
            maxDepthH:           config.maxDepthH ?? 3,
            helpfulnessNeutral:  config.helpfulnessNeutral ?? 0.5,
            helpfulnessFloor:    config.helpfulnessFloor ?? 0.1,
            maxReviewsPerProduct: config.maxReviewsPerProduct ?? 500,
            maxVotesPerReviewer: config.maxVotesPerReviewer ?? 1000,
        };
        this._trustCache = new Map();
    }

    /**
     * @returns {Promise<{
     *   rating: number|null, totalWeight: number,
     *   contributingReviews: number, totalReviewsConsidered: number,
     *   contributions: Array, confidenceRange: number
     * }>}
     */
    async effectiveRating(observer, product, topic) {
        const result = {
            product, topic, observer,
            rating: null,
            totalWeight: 0,
            contributingReviews: 0,
            totalReviewsConsidered: 0,
            contributions: [],
            confidenceRange: 0,
            computedAt: Date.now() / 1000,
        };

        const reviews = await this._fetchReviews(product, topic);
        result.totalReviewsConsidered = reviews.length;

        let weightedSum = 0;
        let totalWeight = 0;

        for (const ev of reviews) {
            const reviewer = ev.creator ?? ev.signerQuid;
            if (!reviewer || reviewer === observer) continue;
            const payload = ev.payload ?? {};
            const rawRating = payload.rating;
            const maxRating = payload.maxRating ?? 5.0;
            if (rawRating == null || !maxRating) continue;

            const normalized = Number(rawRating) / Number(maxRating);

            const t = await this._topicalTrust(observer, reviewer, topic, this.config.maxDepthT);
            if (t <= 0) continue;

            const h = await this._helpfulness(observer, reviewer, topic);
            const a = await this._activity(reviewer);
            const { r, ageDays } = this._recency(ev.timestamp);

            const w = t * h * a * r;
            if (w < this.config.minWeightThreshold) continue;

            result.contributions.push({
                reviewTxId: payload.txId ?? `${ev.subjectId}:${ev.sequence}`,
                reviewerQuid: reviewer,
                rating: normalized * maxRating,
                weight: w,
                t, h, a, r, ageDays,
            });
            weightedSum += normalized * w;
            totalWeight += w;
        }

        result.contributingReviews = result.contributions.length;
        result.totalWeight = totalWeight;

        if (totalWeight > 0) {
            const normalizedAvg = weightedSum / totalWeight;
            result.rating = normalizedAvg * 5.0;
            result.confidenceRange = this._confidenceRange(result.contributions);
        }

        return result;
    }

    // --- Factor T -------------------------------------------------------

    async _topicalTrust(observer, target, topic, maxDepth) {
        const segments = topic.split(".");
        let best = 0;
        let decay = 1;
        for (let depth = segments.length; depth >= 1; depth--) {
            const candidate = segments.slice(0, depth).join(".");
            const score = await this._trustDirect(observer, target, candidate, maxDepth);
            if (score > 0) {
                best = Math.max(best, score * decay);
            }
            decay *= this.config.topicInheritanceDecay;
            if (depth === 2 && best > 0) break;
        }
        return Math.min(best, 1.0);
    }

    async _trustDirect(observer, target, topic, maxDepth) {
        const key = `${observer}|${target}|${topic}|${maxDepth}`;
        if (this._trustCache.has(key)) return this._trustCache.get(key);
        try {
            const tr = await this.client.getTrustLevel(observer, target, topic, { maxDepth });
            const level = Number(tr.trustLevel ?? 0);
            this._trustCache.set(key, level);
            return level;
        } catch {
            this._trustCache.set(key, 0);
            return 0;
        }
    }

    // --- Factor H -------------------------------------------------------

    async _helpfulness(observer, reviewer, topic) {
        let helpfulWeight = 0;
        let unhelpfulWeight = 0;
        let events = [];
        try {
            const res = await this.client.getStreamEvents(reviewer, { limit: this.config.maxVotesPerReviewer });
            events = res.events ?? [];
        } catch {
            return this.config.helpfulnessNeutral;
        }

        for (const ev of events) {
            if (ev.eventType !== "HELPFUL_VOTE" && ev.eventType !== "UNHELPFUL_VOTE") continue;
            const voter = ev.creator ?? ev.signerQuid;
            if (!voter) continue;
            const voterTrust = await this._topicalTrust(observer, voter, topic, this.config.maxDepthH);
            if (voterTrust <= 0) continue;
            if (ev.eventType === "HELPFUL_VOTE") helpfulWeight += voterTrust;
            else unhelpfulWeight += voterTrust;
        }

        const total = helpfulWeight + unhelpfulWeight;
        if (total === 0) return this.config.helpfulnessNeutral;
        const raw = helpfulWeight / total;
        return Math.max(this.config.helpfulnessFloor, raw);
    }

    // --- Factor A -------------------------------------------------------

    async _activity(reviewer) {
        let events = [];
        try {
            const res = await this.client.getStreamEvents(reviewer, { limit: this.config.maxVotesPerReviewer });
            events = res.events ?? [];
        } catch {
            return 0;
        }
        const count = events.filter((e) => e.eventType === "REVIEW").length;
        if (count <= 0) return 0;
        const saturation = Math.max(2, this.config.activitySaturation);
        return Math.min(1, Math.log(count + 1) / Math.log(saturation));
    }

    // --- Factor R -------------------------------------------------------

    _recency(timestamp) {
        if (!timestamp) return { r: this.config.recencyFloor, ageDays: 9999 };
        const ageSeconds = Math.max(0, Date.now() / 1000 - Number(timestamp));
        const ageDays = ageSeconds / 86400;
        const hl = Math.max(1, this.config.recencyHalflifeDays);
        const decayed = Math.exp(-ageDays / hl);
        return { r: Math.max(this.config.recencyFloor, decayed), ageDays };
    }

    // --- Data ---------------------------------------------------------

    async _fetchReviews(product, topic) {
        try {
            const res = await this.client.getStreamEvents(product, {
                domain: topic,
                limit: this.config.maxReviewsPerProduct,
            });
            return (res.events ?? []).filter((e) => e.eventType === "REVIEW");
        } catch {
            return [];
        }
    }

    _confidenceRange(contributions) {
        if (contributions.length === 0) return 1;
        const totalW = contributions.reduce((a, c) => a + c.weight, 0);
        if (totalW === 0) return 1;
        const mean = contributions.reduce((a, c) => a + c.rating * c.weight, 0) / totalW;
        const variance = contributions.reduce(
            (a, c) => a + c.weight * (c.rating - mean) ** 2, 0
        ) / totalW;
        const stddev = Math.sqrt(Math.max(0, variance));
        const effectiveN = Math.max(1, totalW * 2);
        return Math.min(2.5, stddev / Math.sqrt(effectiveN));
    }
}
