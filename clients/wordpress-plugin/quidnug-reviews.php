<?php
/**
 * Plugin Name:       Quidnug Reviews
 * Plugin URI:        https://github.com/bhmortim/quidnug
 * Description:       Drop-in trust-weighted reviews for WordPress + WooCommerce. Per-observer ratings, cross-site reviewer reputation, no proprietary database.
 * Version:           2.0.0
 * Requires at least: 6.2
 * Requires PHP:      7.4
 * Author:            The Quidnug Authors
 * License:           Apache-2.0
 * License URI:       https://www.apache.org/licenses/LICENSE-2.0
 * Text Domain:       quidnug-reviews
 *
 * Architecture:
 *
 * The plugin is intentionally thin. All trust computation and
 * signing happens client-side in the browser via @quidnug/web-components.
 * The plugin:
 *
 *   1. Exposes a settings page for site admins to configure the
 *      node URL + default topic domain.
 *   2. Enqueues the web-components bundle on product / post pages.
 *   3. Emits Schema.org JSON-LD for each product mirroring the
 *      Quidnug review data (via a short-lived cache) for SEO.
 *   4. Replaces the default WooCommerce reviews panel with our
 *      <quidnug-review> element (configurable).
 *   5. Adds a [quidnug-reviews] shortcode for vanilla WP sites.
 *
 * What the plugin does NOT do:
 *
 *   - It doesn't store reviews on the WP database. Reviews live on
 *     the global Quidnug network.
 *   - It doesn't manage reviewer identities. Users sign in via the
 *     Quidnug browser extension or the JS SDK's built-in key mgmt.
 *   - It doesn't implement trust computation server-side. All
 *     per-observer weighting happens in the user's browser.
 */

if (! defined('ABSPATH')) {
    exit;
}

define('QUIDNUG_REVIEWS_VERSION', '2.0.0');
define('QUIDNUG_REVIEWS_PATH', plugin_dir_path(__FILE__));
define('QUIDNUG_REVIEWS_URL',  plugin_dir_url(__FILE__));

require_once QUIDNUG_REVIEWS_PATH . 'includes/class-settings.php';
require_once QUIDNUG_REVIEWS_PATH . 'includes/class-shortcode.php';
require_once QUIDNUG_REVIEWS_PATH . 'includes/class-product-page.php';
require_once QUIDNUG_REVIEWS_PATH . 'includes/class-schema-org.php';
require_once QUIDNUG_REVIEWS_PATH . 'includes/class-enqueue.php';

function quidnug_reviews_init() {
    Quidnug_Reviews\Settings::instance();
    Quidnug_Reviews\Shortcode::instance();
    Quidnug_Reviews\Product_Page::instance();
    Quidnug_Reviews\Schema_Org::instance();
    Quidnug_Reviews\Enqueue::instance();
}

add_action('plugins_loaded', 'quidnug_reviews_init');
