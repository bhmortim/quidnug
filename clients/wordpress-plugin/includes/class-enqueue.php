<?php
namespace Quidnug_Reviews;

if (! defined('ABSPATH')) { exit; }

class Enqueue {
    private static $instance = null;

    public static function instance() {
        if (self::$instance === null) {
            self::$instance = new self();
        }
        return self::$instance;
    }

    private function __construct() {
        add_action('wp_enqueue_scripts', [ $this, 'enqueue' ]);
    }

    public function enqueue() {
        // Load on product / post pages only
        if (! (is_singular(['product', 'post', 'page']) || is_page())) return;

        wp_enqueue_script(
            'quidnug-client',
            'https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client.js',
            [],
            QUIDNUG_REVIEWS_VERSION,
            true
        );

        wp_enqueue_script(
            'quidnug-client-v2',
            'https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client-v2.js',
            [ 'quidnug-client' ],
            QUIDNUG_REVIEWS_VERSION,
            true
        );

        wp_enqueue_script(
            'quidnug-web-components',
            'https://cdn.jsdelivr.net/npm/@quidnug/web-components@2/src/index.js',
            [ 'quidnug-client-v2' ],
            QUIDNUG_REVIEWS_VERSION,
            true
        );

        // Inline bootstrap — set up the client + observer context
        $node_url = esc_js(Settings::instance()->get('node_url'));
        wp_add_inline_script(
            'quidnug-web-components',
            "(async () => {
                const QuidnugClient = (await import('https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client.js')).default;
                await import('https://cdn.jsdelivr.net/npm/@quidnug/client@2/quidnug-client-v2.js');
                const { setClient, setObserverQuid } =
                    await import('https://cdn.jsdelivr.net/npm/@quidnug/web-components@2/src/context.js');

                const client = new QuidnugClient({ defaultNode: '$node_url' });
                setClient(client);

                if (window.quidnug) {
                    try {
                        const quids = await window.quidnug.listQuids();
                        if (quids && quids.length > 0) setObserverQuid(quids[0]);
                    } catch (e) { /* extension not unlocked */ }
                }
            })();"
        );
    }
}
