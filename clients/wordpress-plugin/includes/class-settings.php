<?php
namespace Quidnug_Reviews;

if (! defined('ABSPATH')) { exit; }

class Settings {
    private static $instance = null;

    private $option_group = 'quidnug_reviews_options';
    private $option_name  = 'quidnug_reviews_settings';

    public static function instance() {
        if (self::$instance === null) {
            self::$instance = new self();
        }
        return self::$instance;
    }

    private function __construct() {
        add_action('admin_menu',  [ $this, 'menu' ]);
        add_action('admin_init',  [ $this, 'register_settings' ]);
    }

    public function menu() {
        add_options_page(
            __('Quidnug Reviews', 'quidnug-reviews'),
            __('Quidnug Reviews', 'quidnug-reviews'),
            'manage_options',
            'quidnug-reviews',
            [ $this, 'render_page' ]
        );
    }

    public function register_settings() {
        register_setting($this->option_group, $this->option_name, [
            'sanitize_callback' => [ $this, 'sanitize' ],
            'default' => $this->defaults(),
        ]);

        add_settings_section('quidnug_main', __('Main', 'quidnug-reviews'), null, 'quidnug-reviews');

        add_settings_field('node_url', __('Quidnug node URL', 'quidnug-reviews'),
            [ $this, 'field_text' ], 'quidnug-reviews', 'quidnug_main',
            [ 'key' => 'node_url', 'placeholder' => 'https://public.quidnug.dev' ]);

        add_settings_field('default_topic', __('Default topic domain', 'quidnug-reviews'),
            [ $this, 'field_text' ], 'quidnug-reviews', 'quidnug_main',
            [ 'key' => 'default_topic', 'placeholder' => 'reviews.public.other' ]);

        add_settings_field('replace_woo_reviews', __('Replace WooCommerce reviews', 'quidnug-reviews'),
            [ $this, 'field_checkbox' ], 'quidnug-reviews', 'quidnug_main',
            [ 'key' => 'replace_woo_reviews' ]);

        add_settings_field('emit_schema_org', __('Emit Schema.org JSON-LD for SEO', 'quidnug-reviews'),
            [ $this, 'field_checkbox' ], 'quidnug-reviews', 'quidnug_main',
            [ 'key' => 'emit_schema_org' ]);
    }

    public function render_page() {
        if (! current_user_can('manage_options')) return;
        ?>
        <div class="wrap">
            <h1><?php echo esc_html(get_admin_page_title()); ?></h1>
            <p>Configure Quidnug-reviews settings. Review data lives on the global Quidnug network — this page only configures how your site connects to it.</p>
            <form method="post" action="options.php">
                <?php
                settings_fields($this->option_group);
                do_settings_sections('quidnug-reviews');
                submit_button();
                ?>
            </form>
            <h2><?php esc_html_e('How it works', 'quidnug-reviews'); ?></h2>
            <ul style="list-style: disc; padding-left: 20px;">
                <li>Every product page gets a <code>&lt;quidnug-review&gt;</code> component embedded.</li>
                <li>Visitors see per-observer, trust-weighted star ratings computed against their own Quidnug trust graph.</li>
                <li>Reviewers sign in with the Quidnug browser extension (or any in-page wallet).</li>
                <li>Reviews are posted to the public <code>reviews.public.*</code> domain tree — your site is just rendering the global data.</li>
            </ul>
        </div>
        <?php
    }

    public function field_text($args) {
        $settings = $this->get_all();
        $key = $args['key'];
        $placeholder = $args['placeholder'] ?? '';
        $value = esc_attr($settings[$key] ?? '');
        printf(
            '<input type="text" name="%s[%s]" value="%s" class="regular-text" placeholder="%s" />',
            esc_attr($this->option_name), esc_attr($key), $value, esc_attr($placeholder)
        );
    }

    public function field_checkbox($args) {
        $settings = $this->get_all();
        $key = $args['key'];
        $checked = ! empty($settings[$key]) ? 'checked' : '';
        printf(
            '<input type="checkbox" name="%s[%s]" value="1" %s />',
            esc_attr($this->option_name), esc_attr($key), $checked
        );
    }

    public function sanitize($input) {
        $input = is_array($input) ? $input : [];
        $out = [
            'node_url'            => esc_url_raw($input['node_url'] ?? ''),
            'default_topic'       => sanitize_text_field($input['default_topic'] ?? ''),
            'replace_woo_reviews' => ! empty($input['replace_woo_reviews']),
            'emit_schema_org'     => ! empty($input['emit_schema_org']),
        ];
        return $out;
    }

    public function defaults() {
        return [
            'node_url'            => '',
            'default_topic'       => 'reviews.public.other',
            'replace_woo_reviews' => true,
            'emit_schema_org'     => true,
        ];
    }

    public function get_all() {
        $settings = get_option($this->option_name, $this->defaults());
        return array_merge($this->defaults(), (array) $settings);
    }

    public function get($key, $fallback = null) {
        $s = $this->get_all();
        return $s[$key] ?? $fallback;
    }
}
