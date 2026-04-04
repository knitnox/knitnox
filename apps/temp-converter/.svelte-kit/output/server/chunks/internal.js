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
    app: ({ head, body, assets, nonce, env }) => '<!DOCTYPE html>\r\n<html lang="en">\r\n  <head>\r\n    <meta charset="UTF-8" />\r\n    <meta name="viewport" content="width=device-width, initial-scale=1.0" />\r\n    <title>Temperature Converter — Knitnox</title>\r\n    <meta name="description" content="A free offline temperature converter for Celsius, Fahrenheit, and Kelvin. No ads, no tracking, no internet required. Part of the Knitnox privacy-first PWA platform." />\r\n    <meta name="keywords" content="temperature converter, celsius to fahrenheit, kelvin converter, offline temp tool, pwa, knitnox" />\r\n    <meta name="author" content="Knitnox" />\r\n    <meta name="robots" content="index, follow" />\r\n\r\n    <!-- Open Graph -->\r\n    <meta property="og:type" content="website" />\r\n    <meta property="og:url" content="https://knitnox.github.io/apps/temp-converter/" />\r\n    <meta property="og:site_name" content="Knitnox" />\r\n    <meta property="og:title" content="Temperature Converter — Knitnox" />\r\n    <meta property="og:description" content="Convert between Celsius, Fahrenheit, and Kelvin instantly. Free, offline, no tracking." />\r\n    <meta property="og:image" content="https://knitnox.github.io/icons/og-image.png" />\r\n\r\n    <!-- Twitter / X Card -->\r\n    <meta name="twitter:card" content="summary_large_image" />\r\n    <meta name="twitter:title" content="Temperature Converter — Knitnox" />\r\n    <meta name="twitter:description" content="Convert between Celsius, Fahrenheit, and Kelvin instantly. Free, offline, no tracking." />\r\n    <meta name="twitter:image" content="https://knitnox.github.io/icons/og-image.png" />\r\n\r\n    <!-- Canonical URL -->\r\n    <link rel="canonical" href="https://knitnox.github.io/apps/temp-converter/" />\r\n\r\n    <link rel="icon" href="/favicon.svg" type="image/svg+xml" />\r\n    <link rel="preconnect" href="https://fonts.googleapis.com" />\r\n    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />\r\n    <link href="https://fonts.googleapis.com/css2?family=Orbitron:wght@700;900&display=swap" rel="stylesheet" />\r\n    ' + head + `\r
  <style>\r
    html, body {\r
      margin: 0;\r
      padding: 0;\r
      width: 100%;\r
      height: 100%;\r
    }\r
    #splash {\r
      position: fixed;\r
      inset: 0;\r
      background: #e6e9ef;\r
      display: flex;\r
      align-items: center;\r
      justify-content: center;\r
      z-index: 9999;\r
      transition: opacity 0.45s ease, visibility 0.45s ease;\r
    }\r
    #splash.hidden {\r
      opacity: 0;\r
      visibility: hidden;\r
    }\r
    .splash-card {\r
      display: flex;\r
      flex-direction: column;\r
      align-items: center;\r
      gap: 18px;\r
      padding: 40px 48px;\r
      border-radius: 28px;\r
      background: #e6e9ef;\r
      box-shadow: 10px 10px 20px #c2c8d0, -10px -10px 20px #ffffff;\r
      animation: splash-pop 0.4s cubic-bezier(0.34,1.56,0.64,1) both;\r
    }\r
    @keyframes splash-pop {\r
      from { transform: scale(0.85); opacity: 0; }\r
      to   { transform: scale(1);    opacity: 1; }\r
    }\r
    .splash-icon { width: 88px; height: 88px; }\r
    .splash-name {\r
      font-family: 'Orbitron', sans-serif;\r
      font-size: 1.5rem;\r
      font-weight: 700;\r
      color: #2a2f3a;\r
      margin: 0;\r
      letter-spacing: 0.04em;\r
    }\r
    .splash-brand {\r
      font-size: 0.8rem;\r
      font-weight: 600;\r
      color: #8a9ab0;\r
      margin: 0;\r
      letter-spacing: 0.18em;\r
      text-transform: uppercase;\r
    }\r
  </style>\r
  </head>\r
  <body>\r
    <div id="splash" aria-hidden="true">\r
      <div class="splash-card">\r
        <img src="` + assets + '/icon.svg" alt="" class="splash-icon" />\r\n        <p class="splash-name">Temp Converter</p>\r\n        <p class="splash-brand">Knitnox</p>\r\n      </div>\r\n    </div>\r\n    <div style="display: contents">' + body + "</div>\r\n  </body>\r\n</html>\r\n",
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
  version_hash: "i6e2fz"
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
