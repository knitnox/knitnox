import { r as root } from "./root.js";
import "./environment.js";
let public_env = {};
function set_private_env(environment) {
}
function set_public_env(environment) {
  public_env = environment;
}
let read_implementation = null;
function set_read_implementation(fn) {
  read_implementation = fn;
}
function set_manifest(_) {
}
const options = {
  app_template_contains_nonce: false,
  async: false,
  csp: { "mode": "auto", "directives": { "upgrade-insecure-requests": false, "block-all-mixed-content": false }, "reportOnly": { "upgrade-insecure-requests": false, "block-all-mixed-content": false } },
  csrf_check_origin: true,
  csrf_trusted_origins: [],
  embedded: false,
  env_public_prefix: "PUBLIC_",
  env_private_prefix: "",
  hash_routing: false,
  hooks: null,
  // added lazily, via `get_hooks`
  preload_strategy: "modulepreload",
  root,
  service_worker: false,
  service_worker_options: void 0,
  server_error_boundaries: false,
  templates: {
    app: ({ head, body, assets, nonce, env }) => '<!DOCTYPE html>\r\n<html lang="en">\r\n  <head>\r\n    <meta charset="UTF-8" />\r\n    <meta name="viewport" content="width=device-width, initial-scale=1.0" />\r\n    <title>Knitnox — App for Hardware Control & Privacy-First Tools</title>\r\n    <meta name="description" content="Hardware control &amp; privacy-first tools — zero cloud, zero latency. Browser-native control for ESP32, Arduino &amp; Raspberry Pi plus offline-first PWA utilities." />\r\n    <meta name="keywords" content="knitnox, web bluetooth, web serial, webusb, esp32, arduino, raspberry pi, pwa, offline apps, browser hardware control" />\r\n    <meta name="author" content="Knitnox" />\r\n    <meta name="robots" content="index, follow" />\r\n\r\n    <!-- Open Graph -->\r\n    <meta property="og:type" content="website" />\r\n    <meta property="og:url" content="https://knitnox.github.io/" />\r\n    <meta property="og:site_name" content="Knitnox" />\r\n    <meta property="og:title" content="Knitnox — App for Hardware Control & Privacy-First Tools" />\r\n    <meta property="og:description" content="Hardware control &amp; privacy-first tools — zero cloud, zero latency." />\r\n    <meta property="og:image" content="https://knitnox.github.io/icons/og-image.png" />\r\n\r\n    <!-- Twitter / X Card -->\r\n    <meta name="twitter:card" content="summary_large_image" />\r\n    <meta name="twitter:title" content="Knitnox — App for Hardware Control & Privacy-First Tools" />\r\n    <meta name="twitter:description" content="Hardware control &amp; privacy-first tools — zero cloud, zero latency." />\r\n    <meta name="twitter:image" content="https://knitnox.github.io/icons/og-image.png" />\r\n\r\n    <!-- Canonical URL -->\r\n    <link rel="canonical" href="https://knitnox.github.io/" />\r\n\r\n    <link rel="icon" href="/favicon.svg" type="image/svg+xml" />\r\n    <link rel="preconnect" href="https://fonts.googleapis.com" />\r\n    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />\r\n    <link href="https://fonts.googleapis.com/css2?family=Orbitron:wght@700;900&display=swap" rel="stylesheet" />\r\n\r\n    ' + head + `\r
\r
    <script>\r
      // Backward compat: restore path redirected from the old GitHub Pages 404.html\r
      (function () {\r
        var q = window.location.search;\r
        if (q && q.indexOf('?p=') === 0) {\r
          var path = decodeURIComponent(q.slice(3));\r
          if (/^\\/apps\\/[^/]/.test(path)) {\r
            window.location.replace(path);\r
          } else {\r
            window.history.replaceState(null, null, path + window.location.hash);\r
          }\r
        }\r
      })();\r
    <\/script>\r
  </head>\r
  <body data-sveltekit-preload-data="hover">\r
    <div id="splash" aria-hidden="true">\r
      <span>K</span><span>n</span><span>i</span><span>t</span><span>n</span><span class="glow">o</span><span>x</span>\r
    </div>\r
    <style>\r
      #splash {\r
        position: fixed;\r
        inset: 0;\r
        z-index: 9999;\r
        display: flex;\r
        align-items: center;\r
        justify-content: center;\r
        gap: 0;\r
        background: #e6e9ef;\r
        font-family: 'Orbitron', sans-serif;\r
        font-size: clamp(2rem, 10vw, 3.2rem);\r
        font-weight: 900;\r
        letter-spacing: 4px;\r
        color: #3a3f5c;\r
        pointer-events: none;\r
        animation: splashExit 0.5s cubic-bezier(0.4, 0, 0.2, 1) 1.2s forwards;\r
      }\r
      #splash .glow {\r
        color: #ffa500;\r
        animation: bulbGlow 2s ease-in-out infinite;\r
      }\r
      @keyframes splashExit {\r
        to { transform: translateY(-100%); opacity: 0; }\r
      }\r
      @keyframes bulbGlow {\r
        0%, 100% {\r
          text-shadow: 0 0 10px rgba(255,200,0,0.4),\r
                       0 0 20px rgba(255,165,0,0.5),\r
                       0 0 30px rgba(255,140,0,0.3);\r
        }\r
        50% {\r
          text-shadow: 0 0 20px rgba(255,200,0,0.8),\r
                       0 0 35px rgba(255,165,0,0.9),\r
                       0 0 50px rgba(255,140,0,0.6),\r
                       0 0 70px rgba(255,120,0,0.4);\r
        }\r
      }\r
    </style>\r
    <div style="display: contents">` + body + "</div>\r\n  </body>\r\n</html>\r\n",
    error: ({ status, message }) => '<!doctype html>\n<html lang="en">\n	<head>\n		<meta charset="utf-8" />\n		<title>' + message + `</title>

		<style>
			body {
				--bg: white;
				--fg: #222;
				--divider: #ccc;
				background: var(--bg);
				color: var(--fg);
				font-family:
					system-ui,
					-apple-system,
					BlinkMacSystemFont,
					'Segoe UI',
					Roboto,
					Oxygen,
					Ubuntu,
					Cantarell,
					'Open Sans',
					'Helvetica Neue',
					sans-serif;
				display: flex;
				align-items: center;
				justify-content: center;
				height: 100vh;
				margin: 0;
			}

			.error {
				display: flex;
				align-items: center;
				max-width: 32rem;
				margin: 0 1rem;
			}

			.status {
				font-weight: 200;
				font-size: 3rem;
				line-height: 1;
				position: relative;
				top: -0.05rem;
			}

			.message {
				border-left: 1px solid var(--divider);
				padding: 0 0 0 1rem;
				margin: 0 0 0 1rem;
				min-height: 2.5rem;
				display: flex;
				align-items: center;
			}

			.message h1 {
				font-weight: 400;
				font-size: 1em;
				margin: 0;
			}

			@media (prefers-color-scheme: dark) {
				body {
					--bg: #222;
					--fg: #ddd;
					--divider: #666;
				}
			}
		</style>
	</head>
	<body>
		<div class="error">
			<span class="status">` + status + '</span>\n			<div class="message">\n				<h1>' + message + "</h1>\n			</div>\n		</div>\n	</body>\n</html>\n"
  },
  version_hash: "urz27y"
};
async function get_hooks() {
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
export {
  set_public_env as a,
  set_read_implementation as b,
  set_manifest as c,
  get_hooks as g,
  options as o,
  public_env as p,
  read_implementation as r,
  set_private_env as s
};
