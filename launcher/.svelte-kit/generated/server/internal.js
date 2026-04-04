
import root from '../root.js';
import { set_building, set_prerendering } from '__sveltekit/environment';
import { set_assets } from '$app/paths/internal/server';
import { set_manifest, set_read_implementation } from '__sveltekit/server';
import { set_private_env, set_public_env } from '../../../node_modules/@sveltejs/kit/src/runtime/shared-server.js';

export const options = {
	app_template_contains_nonce: false,
	async: false,
	csp: {"mode":"auto","directives":{"upgrade-insecure-requests":false,"block-all-mixed-content":false},"reportOnly":{"upgrade-insecure-requests":false,"block-all-mixed-content":false}},
	csrf_check_origin: true,
	csrf_trusted_origins: [],
	embedded: false,
	env_public_prefix: 'PUBLIC_',
	env_private_prefix: '',
	hash_routing: false,
	hooks: null, // added lazily, via `get_hooks`
	preload_strategy: "modulepreload",
	root,
	service_worker: false,
	service_worker_options: undefined,
	server_error_boundaries: false,
	templates: {
		app: ({ head, body, assets, nonce, env }) => "<!DOCTYPE html>\r\n<html lang=\"en\">\r\n  <head>\r\n    <meta charset=\"UTF-8\" />\r\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\r\n    <title>Knitnox — Your browser OS.</title>\r\n    <meta name=\"description\" content=\"Hardware control &amp; privacy-first tools — zero cloud, zero latency. Browser-native control for ESP32, Arduino &amp; Raspberry Pi plus offline-first PWA utilities.\" />\r\n    <meta name=\"keywords\" content=\"knitnox, web bluetooth, web serial, webusb, esp32, arduino, raspberry pi, pwa, offline apps, browser hardware control\" />\r\n    <meta name=\"author\" content=\"Knitnox\" />\r\n    <meta name=\"robots\" content=\"index, follow\" />\r\n\r\n    <!-- Open Graph -->\r\n    <meta property=\"og:type\" content=\"website\" />\r\n    <meta property=\"og:url\" content=\"https://knitnox.github.io/\" />\r\n    <meta property=\"og:site_name\" content=\"Knitnox\" />\r\n    <meta property=\"og:title\" content=\"Knitnox — Your browser is the controller.\" />\r\n    <meta property=\"og:description\" content=\"Hardware control &amp; privacy-first tools — zero cloud, zero latency.\" />\r\n    <meta property=\"og:image\" content=\"https://knitnox.github.io/icons/og-image.png\" />\r\n\r\n    <!-- Twitter / X Card -->\r\n    <meta name=\"twitter:card\" content=\"summary_large_image\" />\r\n    <meta name=\"twitter:title\" content=\"Knitnox — Your browser is the controller.\" />\r\n    <meta name=\"twitter:description\" content=\"Hardware control &amp; privacy-first tools — zero cloud, zero latency.\" />\r\n    <meta name=\"twitter:image\" content=\"https://knitnox.github.io/icons/og-image.png\" />\r\n\r\n    <!-- Canonical URL -->\r\n    <link rel=\"canonical\" href=\"https://knitnox.github.io/\" />\r\n\r\n    <link rel=\"icon\" href=\"/favicon.svg\" type=\"image/svg+xml\" />\r\n    <link rel=\"preconnect\" href=\"https://fonts.googleapis.com\" />\r\n    <link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin />\r\n    <link href=\"https://fonts.googleapis.com/css2?family=Orbitron:wght@700;900&display=swap\" rel=\"stylesheet\" />\r\n\r\n    " + head + "\r\n\r\n    <script>\r\n      // Backward compat: restore path redirected from the old GitHub Pages 404.html\r\n      (function () {\r\n        var q = window.location.search;\r\n        if (q && q.indexOf('?p=') === 0) {\r\n          var path = decodeURIComponent(q.slice(3));\r\n          if (/^\\/apps\\/[^/]/.test(path)) {\r\n            window.location.replace(path);\r\n          } else {\r\n            window.history.replaceState(null, null, path + window.location.hash);\r\n          }\r\n        }\r\n      })();\r\n    </script>\r\n  </head>\r\n  <body data-sveltekit-preload-data=\"hover\">\r\n    <div id=\"splash\" aria-hidden=\"true\">\r\n      <span>K</span><span>n</span><span>i</span><span>t</span><span>n</span><span class=\"glow\">o</span><span>x</span>\r\n    </div>\r\n    <style>\r\n      #splash {\r\n        position: fixed;\r\n        inset: 0;\r\n        z-index: 9999;\r\n        display: flex;\r\n        align-items: center;\r\n        justify-content: center;\r\n        gap: 0;\r\n        background: #e6e9ef;\r\n        font-family: 'Orbitron', sans-serif;\r\n        font-size: clamp(2rem, 10vw, 3.2rem);\r\n        font-weight: 900;\r\n        letter-spacing: 4px;\r\n        color: #3a3f5c;\r\n        pointer-events: none;\r\n        animation: splashExit 0.5s cubic-bezier(0.4, 0, 0.2, 1) 1.2s forwards;\r\n      }\r\n      #splash .glow {\r\n        color: #ffa500;\r\n        animation: bulbGlow 2s ease-in-out infinite;\r\n      }\r\n      @keyframes splashExit {\r\n        to { transform: translateY(-100%); opacity: 0; }\r\n      }\r\n      @keyframes bulbGlow {\r\n        0%, 100% {\r\n          text-shadow: 0 0 10px rgba(255,200,0,0.4),\r\n                       0 0 20px rgba(255,165,0,0.5),\r\n                       0 0 30px rgba(255,140,0,0.3);\r\n        }\r\n        50% {\r\n          text-shadow: 0 0 20px rgba(255,200,0,0.8),\r\n                       0 0 35px rgba(255,165,0,0.9),\r\n                       0 0 50px rgba(255,140,0,0.6),\r\n                       0 0 70px rgba(255,120,0,0.4);\r\n        }\r\n      }\r\n    </style>\r\n    <div style=\"display: contents\">" + body + "</div>\r\n  </body>\r\n</html>\r\n",
		error: ({ status, message }) => "<!doctype html>\n<html lang=\"en\">\n\t<head>\n\t\t<meta charset=\"utf-8\" />\n\t\t<title>" + message + "</title>\n\n\t\t<style>\n\t\t\tbody {\n\t\t\t\t--bg: white;\n\t\t\t\t--fg: #222;\n\t\t\t\t--divider: #ccc;\n\t\t\t\tbackground: var(--bg);\n\t\t\t\tcolor: var(--fg);\n\t\t\t\tfont-family:\n\t\t\t\t\tsystem-ui,\n\t\t\t\t\t-apple-system,\n\t\t\t\t\tBlinkMacSystemFont,\n\t\t\t\t\t'Segoe UI',\n\t\t\t\t\tRoboto,\n\t\t\t\t\tOxygen,\n\t\t\t\t\tUbuntu,\n\t\t\t\t\tCantarell,\n\t\t\t\t\t'Open Sans',\n\t\t\t\t\t'Helvetica Neue',\n\t\t\t\t\tsans-serif;\n\t\t\t\tdisplay: flex;\n\t\t\t\talign-items: center;\n\t\t\t\tjustify-content: center;\n\t\t\t\theight: 100vh;\n\t\t\t\tmargin: 0;\n\t\t\t}\n\n\t\t\t.error {\n\t\t\t\tdisplay: flex;\n\t\t\t\talign-items: center;\n\t\t\t\tmax-width: 32rem;\n\t\t\t\tmargin: 0 1rem;\n\t\t\t}\n\n\t\t\t.status {\n\t\t\t\tfont-weight: 200;\n\t\t\t\tfont-size: 3rem;\n\t\t\t\tline-height: 1;\n\t\t\t\tposition: relative;\n\t\t\t\ttop: -0.05rem;\n\t\t\t}\n\n\t\t\t.message {\n\t\t\t\tborder-left: 1px solid var(--divider);\n\t\t\t\tpadding: 0 0 0 1rem;\n\t\t\t\tmargin: 0 0 0 1rem;\n\t\t\t\tmin-height: 2.5rem;\n\t\t\t\tdisplay: flex;\n\t\t\t\talign-items: center;\n\t\t\t}\n\n\t\t\t.message h1 {\n\t\t\t\tfont-weight: 400;\n\t\t\t\tfont-size: 1em;\n\t\t\t\tmargin: 0;\n\t\t\t}\n\n\t\t\t@media (prefers-color-scheme: dark) {\n\t\t\t\tbody {\n\t\t\t\t\t--bg: #222;\n\t\t\t\t\t--fg: #ddd;\n\t\t\t\t\t--divider: #666;\n\t\t\t\t}\n\t\t\t}\n\t\t</style>\n\t</head>\n\t<body>\n\t\t<div class=\"error\">\n\t\t\t<span class=\"status\">" + status + "</span>\n\t\t\t<div class=\"message\">\n\t\t\t\t<h1>" + message + "</h1>\n\t\t\t</div>\n\t\t</div>\n\t</body>\n</html>\n"
	},
	version_hash: "odhe3b"
};

export async function get_hooks() {
	let handle;
	let handleFetch;
	let handleError;
	let handleValidationError;
	let init;
	

	let reroute;
	let transport;
	

	return {
		handle,
		handleFetch,
		handleError,
		handleValidationError,
		init,
		reroute,
		transport
	};
}

export { set_assets, set_building, set_manifest, set_prerendering, set_private_env, set_public_env, set_read_implementation };
