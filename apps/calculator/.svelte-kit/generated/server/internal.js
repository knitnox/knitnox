
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
		app: ({ head, body, assets, nonce, env }) => "<!DOCTYPE html>\r\n<html lang=\"en\">\r\n  <head>\r\n    <meta charset=\"UTF-8\" />\r\n    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\" />\r\n    <title>Calculator — Knitnox</title>\r\n    <meta name=\"description\" content=\"A free offline calculator that runs entirely in your browser. No ads, no tracking, no internet required. Part of the Knitnox privacy-first PWA platform.\" />\r\n    <meta name=\"keywords\" content=\"calculator, offline calculator, pwa calculator, browser calculator, knitnox, privacy calculator\" />\r\n    <meta name=\"author\" content=\"Knitnox\" />\r\n    <meta name=\"robots\" content=\"index, follow\" />\r\n\r\n    <!-- Open Graph -->\r\n    <meta property=\"og:type\" content=\"website\" />\r\n    <meta property=\"og:url\" content=\"https://knitnox.github.io/apps/calculator/\" />\r\n    <meta property=\"og:site_name\" content=\"Knitnox\" />\r\n    <meta property=\"og:title\" content=\"Calculator — Knitnox\" />\r\n    <meta property=\"og:description\" content=\"A free offline calculator. No ads, no tracking, no internet required.\" />\r\n    <meta property=\"og:image\" content=\"https://knitnox.github.io/icons/og-image.png\" />\r\n\r\n    <!-- Twitter / X Card -->\r\n    <meta name=\"twitter:card\" content=\"summary_large_image\" />\r\n    <meta name=\"twitter:title\" content=\"Calculator — Knitnox\" />\r\n    <meta name=\"twitter:description\" content=\"A free offline calculator. No ads, no tracking, no internet required.\" />\r\n    <meta name=\"twitter:image\" content=\"https://knitnox.github.io/icons/og-image.png\" />\r\n\r\n    <!-- Canonical URL -->\r\n    <link rel=\"canonical\" href=\"https://knitnox.github.io/apps/calculator/\" />\r\n\r\n    <link rel=\"icon\" href=\"/favicon.svg\" type=\"image/svg+xml\" />\r\n    <link rel=\"preconnect\" href=\"https://fonts.googleapis.com\" />\r\n    <link rel=\"preconnect\" href=\"https://fonts.gstatic.com\" crossorigin />\r\n    <link href=\"https://fonts.googleapis.com/css2?family=Orbitron:wght@700;900&display=swap\" rel=\"stylesheet\" />\r\n\r\n    " + head + "\r\n\r\n    <style>\r\n      html, body { margin: 0; padding: 0; width: 100%; height: 100%; }\r\n      #app { width: 100%; min-height: 100vh; min-height: 100dvh; display: flex; flex-direction: column; }\r\n      #splash {\r\n        position: fixed; inset: 0; background: #e6e9ef;\r\n        display: flex; align-items: center; justify-content: center;\r\n        z-index: 9999; transition: opacity 0.45s ease, visibility 0.45s ease;\r\n      }\r\n      #splash.hidden { opacity: 0; visibility: hidden; }\r\n      .splash-card {\r\n        display: flex; flex-direction: column; align-items: center; gap: 18px;\r\n        padding: 40px 48px; border-radius: 28px; background: #e6e9ef;\r\n        box-shadow: 10px 10px 20px #c2c8d0, -10px -10px 20px #ffffff;\r\n        animation: splash-pop 0.4s cubic-bezier(0.34,1.56,0.64,1) both;\r\n      }\r\n      @keyframes splash-pop { from { transform: scale(0.85); opacity: 0; } to { transform: scale(1); opacity: 1; } }\r\n      .splash-icon { width: 88px; height: 88px; }\r\n      .splash-name { font-family: 'Orbitron', sans-serif; font-size: 1.5rem; font-weight: 700; color: #2a2f3a; margin: 0; letter-spacing: 0.04em; }\r\n      .splash-brand { font-size: 0.8rem; font-weight: 600; color: #8a9ab0; margin: 0; letter-spacing: 0.18em; text-transform: uppercase; }\r\n    </style>\r\n  </head>\r\n  <body>\r\n    <div id=\"splash\" aria-hidden=\"true\">\r\n      <div class=\"splash-card\">\r\n        <img src=\"" + assets + "/icon.svg\" alt=\"\" class=\"splash-icon\" />\r\n        <p class=\"splash-name\">Calculator</p>\r\n        <p class=\"splash-brand\">Knitnox</p>\r\n      </div>\r\n    </div>\r\n    <div style=\"display: contents\">" + body + "</div>\r\n  </body>\r\n</html>\r\n",
		error: ({ status, message }) => "<!doctype html>\n<html lang=\"en\">\n\t<head>\n\t\t<meta charset=\"utf-8\" />\n\t\t<title>" + message + "</title>\n\n\t\t<style>\n\t\t\tbody {\n\t\t\t\t--bg: white;\n\t\t\t\t--fg: #222;\n\t\t\t\t--divider: #ccc;\n\t\t\t\tbackground: var(--bg);\n\t\t\t\tcolor: var(--fg);\n\t\t\t\tfont-family:\n\t\t\t\t\tsystem-ui,\n\t\t\t\t\t-apple-system,\n\t\t\t\t\tBlinkMacSystemFont,\n\t\t\t\t\t'Segoe UI',\n\t\t\t\t\tRoboto,\n\t\t\t\t\tOxygen,\n\t\t\t\t\tUbuntu,\n\t\t\t\t\tCantarell,\n\t\t\t\t\t'Open Sans',\n\t\t\t\t\t'Helvetica Neue',\n\t\t\t\t\tsans-serif;\n\t\t\t\tdisplay: flex;\n\t\t\t\talign-items: center;\n\t\t\t\tjustify-content: center;\n\t\t\t\theight: 100vh;\n\t\t\t\tmargin: 0;\n\t\t\t}\n\n\t\t\t.error {\n\t\t\t\tdisplay: flex;\n\t\t\t\talign-items: center;\n\t\t\t\tmax-width: 32rem;\n\t\t\t\tmargin: 0 1rem;\n\t\t\t}\n\n\t\t\t.status {\n\t\t\t\tfont-weight: 200;\n\t\t\t\tfont-size: 3rem;\n\t\t\t\tline-height: 1;\n\t\t\t\tposition: relative;\n\t\t\t\ttop: -0.05rem;\n\t\t\t}\n\n\t\t\t.message {\n\t\t\t\tborder-left: 1px solid var(--divider);\n\t\t\t\tpadding: 0 0 0 1rem;\n\t\t\t\tmargin: 0 0 0 1rem;\n\t\t\t\tmin-height: 2.5rem;\n\t\t\t\tdisplay: flex;\n\t\t\t\talign-items: center;\n\t\t\t}\n\n\t\t\t.message h1 {\n\t\t\t\tfont-weight: 400;\n\t\t\t\tfont-size: 1em;\n\t\t\t\tmargin: 0;\n\t\t\t}\n\n\t\t\t@media (prefers-color-scheme: dark) {\n\t\t\t\tbody {\n\t\t\t\t\t--bg: #222;\n\t\t\t\t\t--fg: #ddd;\n\t\t\t\t\t--divider: #666;\n\t\t\t\t}\n\t\t\t}\n\t\t</style>\n\t</head>\n\t<body>\n\t\t<div class=\"error\">\n\t\t\t<span class=\"status\">" + status + "</span>\n\t\t\t<div class=\"message\">\n\t\t\t\t<h1>" + message + "</h1>\n\t\t\t</div>\n\t\t</div>\n\t</body>\n</html>\n"
	},
	version_hash: "1n33867"
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
