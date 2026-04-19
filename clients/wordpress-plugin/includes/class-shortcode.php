<?php
namespace Quidnug_Reviews;

if (! defined('ABSPATH')) { exit; }

/**
 * [quidnug-reviews product="..." topic="..." show-write="1"]
 *
 * Drop anywhere in a post or page to render a full review panel.
 * Also supports just stars:
 *
 * [quidnug-stars product="..." topic="..."]
 */
class Shortcode {
    private static $instance = null;

    public static function instance() {
        if (self::$instance === null) {
            self::$instance = new self();
        }
        return self::$instance;
    }

    private function __construct() {
        add_shortcode('quidnug-reviews', [ $this, 'reviews' ]);
        add_shortcode('quidnug-stars',   [ $this, 'stars' ]);
    }

    public function reviews($atts) {
        $a = shortcode_atts([
            'product'    => '',
            'topic'      => Settings::instance()->get('default_topic'),
            'show-write' => '0',
        ], $atts, 'quidnug-reviews');

        if (empty($a['product'])) return '';

        $show_write = ! empty($a['show-write']) && $a['show-write'] !== '0' ? ' show-write' : '';

        return sprintf(
            '<quidnug-review product="%s" topic="%s"%s></quidnug-review>',
            esc_attr($a['product']), esc_attr($a['topic']), $show_write
        );
    }

    public function stars($atts) {
        $a = shortcode_atts([
            'product'    => '',
            'topic'      => Settings::instance()->get('default_topic'),
            'show-count' => '0',
        ], $atts, 'quidnug-stars');

        if (empty($a['product'])) return '';

        $show_count = ! empty($a['show-count']) && $a['show-count'] !== '0' ? ' show-count' : '';

        return sprintf(
            '<quidnug-stars product="%s" topic="%s"%s></quidnug-stars>',
            esc_attr($a['product']), esc_attr($a['topic']), $show_count
        );
    }
}
