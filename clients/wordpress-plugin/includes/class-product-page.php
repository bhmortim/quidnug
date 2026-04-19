<?php
namespace Quidnug_Reviews;

if (! defined('ABSPATH')) { exit; }

/**
 * WooCommerce integration: replace the default product review tab
 * with Quidnug reviews, using the product's SKU / ID as the canonical
 * product identifier.
 *
 * Gated behind the "replace_woo_reviews" setting.
 */
class Product_Page {
    private static $instance = null;

    public static function instance() {
        if (self::$instance === null) {
            self::$instance = new self();
        }
        return self::$instance;
    }

    private function __construct() {
        // Only take effect if WooCommerce is active
        add_action('init', [ $this, 'maybe_hook_woo' ]);
    }

    public function maybe_hook_woo() {
        if (! class_exists('WooCommerce')) return;
        if (! Settings::instance()->get('replace_woo_reviews')) return;

        // Replace the reviews tab content
        add_filter('woocommerce_product_tabs', [ $this, 'filter_tabs' ], 999);
    }

    public function filter_tabs($tabs) {
        if (isset($tabs['reviews'])) {
            $tabs['reviews']['callback'] = [ $this, 'render_reviews_tab' ];
            $tabs['reviews']['title']    = __('Trust-weighted reviews', 'quidnug-reviews');
        }
        return $tabs;
    }

    public function render_reviews_tab() {
        global $product;
        if (! $product) return;

        $topic = $this->topic_for_product($product);
        $asset = $this->asset_id_for_product($product);

        printf(
            '<quidnug-review product="%s" topic="%s" show-write></quidnug-review>',
            esc_attr($asset), esc_attr($topic)
        );
    }

    private function asset_id_for_product($product) {
        // Canonical id: SKU if set, else permalink hash
        $sku = $product->get_sku();
        if ($sku) {
            return 'wc-' . sanitize_key($sku);
        }
        return 'wc-' . substr(hash('sha256', $product->get_permalink()), 0, 16);
    }

    private function topic_for_product($product) {
        // Simple category-to-topic mapping. Sites can override via a filter.
        $cats = wp_get_post_terms($product->get_id(), 'product_cat', [ 'fields' => 'slugs' ]);
        if (empty($cats)) {
            return Settings::instance()->get('default_topic');
        }

        $first = sanitize_key($cats[0]);
        $mapping = apply_filters('quidnug_reviews_category_topic_map', [
            'electronics' => 'reviews.public.technology',
            'computers'   => 'reviews.public.technology.laptops',
            'cameras'     => 'reviews.public.technology.cameras',
            'phones'      => 'reviews.public.technology.phones',
            'books'       => 'reviews.public.books',
            'movies'      => 'reviews.public.movies',
            'food'        => 'reviews.public.restaurants',
        ]);

        return $mapping[$first] ?? (Settings::instance()->get('default_topic') . '.' . $first);
    }
}
