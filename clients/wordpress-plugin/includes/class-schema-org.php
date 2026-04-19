<?php
namespace Quidnug_Reviews;

if (! defined('ABSPATH')) { exit; }

/**
 * Emit Schema.org Review JSON-LD for SEO compatibility with
 * Google / Bing / DuckDuckGo rich results.
 *
 * The emitted JSON-LD reflects the *aggregate* Quidnug ratings,
 * computed server-side using a cached public-observer view. Per-
 * observer weighting still happens client-side for the interactive
 * display — the JSON-LD is for search-engine crawlers, which are
 * de facto "anonymous" observers.
 */
class Schema_Org {
    private static $instance = null;

    public static function instance() {
        if (self::$instance === null) {
            self::$instance = new self();
        }
        return self::$instance;
    }

    private function __construct() {
        add_action('wp_head', [ $this, 'maybe_emit' ], 99);
    }

    public function maybe_emit() {
        if (! Settings::instance()->get('emit_schema_org')) return;
        if (! is_singular(['product', 'post', 'page'])) return;

        $product_id = $this->product_id_for_current();
        $topic      = $this->topic_for_current();

        if (! $product_id) return;

        $aggregate = $this->fetch_aggregate($product_id, $topic);
        if (! $aggregate) return;

        $json = [
            '@context' => 'https://schema.org',
            '@type'    => 'AggregateRating',
            'itemReviewed' => [
                '@type' => 'Product',
                'name'  => get_the_title(),
                'url'   => get_permalink(),
            ],
            'ratingValue'  => (float) $aggregate['rating'],
            'reviewCount'  => (int) $aggregate['contributingReviews'],
            'bestRating'   => 5,
            'worstRating'  => 0,
        ];

        printf(
            "\n<script type=\"application/ld+json\">%s</script>\n",
            wp_json_encode($json, JSON_UNESCAPED_SLASHES)
        );
    }

    private function product_id_for_current() {
        $post_id = get_queried_object_id();
        if (! $post_id) return null;

        // WooCommerce product: use SKU if present
        if (function_exists('wc_get_product')) {
            $wc_product = wc_get_product($post_id);
            if ($wc_product) {
                $sku = $wc_product->get_sku();
                if ($sku) return 'wc-' . sanitize_key($sku);
            }
        }

        return 'wp-' . substr(hash('sha256', get_permalink($post_id)), 0, 16);
    }

    private function topic_for_current() {
        return Settings::instance()->get('default_topic');
    }

    /**
     * Fetch the aggregate "anonymous observer" rating from the
     * Quidnug node. Cached for 5 minutes.
     *
     * Returns array | null.
     */
    private function fetch_aggregate($product, $topic) {
        $cache_key = 'qng_agg_' . md5("$product|$topic");
        $cached    = get_transient($cache_key);
        if ($cached !== false) return $cached;

        $node_url = Settings::instance()->get('node_url');
        if (empty($node_url)) return null;

        // Fetch the raw event stream; compute a simple anonymous average
        // (the per-observer view runs in the browser, not here).
        $url = trailingslashit($node_url) . 'api/streams/' . urlencode($product)
             . '/events?domain=' . urlencode($topic) . '&limit=200';

        $response = wp_remote_get($url, [ 'timeout' => 8 ]);
        if (is_wp_error($response) || wp_remote_retrieve_response_code($response) !== 200) {
            return null;
        }

        $body = json_decode(wp_remote_retrieve_body($response), true);
        $events = $body['data']['data'] ?? $body['data']['events'] ?? $body['data'] ?? [];
        if (! is_array($events)) return null;

        $count = 0;
        $sum = 0.0;
        foreach ($events as $ev) {
            if (($ev['eventType'] ?? '') !== 'REVIEW') continue;
            $rating = $ev['payload']['rating'] ?? null;
            $max    = $ev['payload']['maxRating'] ?? 5.0;
            if ($rating === null || ! $max) continue;
            $sum += ($rating / $max) * 5.0;
            $count++;
        }

        if ($count === 0) return null;

        $agg = [
            'rating'              => round($sum / $count, 2),
            'contributingReviews' => $count,
        ];

        set_transient($cache_key, $agg, 5 * MINUTE_IN_SECONDS);
        return $agg;
    }
}
