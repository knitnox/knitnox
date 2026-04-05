export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "apps/calculator/_app",
	assets: new Set(["favicon.svg","icon.svg","screenshots/screenshot.PNG","screenshots/screenshot2.PNG"]),
	mimeTypes: {".svg":"image/svg+xml",".PNG":"image/png"},
	_: {
		client: {start:"_app/immutable/entry/start.DTgEGcPi.js",app:"_app/immutable/entry/app.UhATShXl.js",imports:["_app/immutable/entry/start.DTgEGcPi.js","_app/immutable/chunks/BYRpRWqK.js","_app/immutable/chunks/BzxVPEC4.js","_app/immutable/chunks/UyyJJpq1.js","_app/immutable/entry/app.UhATShXl.js","_app/immutable/chunks/BzxVPEC4.js","_app/immutable/chunks/Cajzzv9o.js","_app/immutable/chunks/nK2oQkYf.js","_app/immutable/chunks/UyyJJpq1.js","_app/immutable/chunks/D8KGAZC3.js","_app/immutable/chunks/BWdWBhQN.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js'))
		],
		remotes: {
			
		},
		routes: [
			{
				id: "/",
				pattern: /^\/$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 2 },
				endpoint: null
			}
		],
		prerendered_routes: new Set([]),
		matchers: async () => {
			
			return {  };
		},
		server_assets: {}
	}
}
})();
