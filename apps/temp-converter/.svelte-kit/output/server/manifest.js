export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "apps/temp-converter/_app",
	assets: new Set(["favicon.svg","icon.svg"]),
	mimeTypes: {".svg":"image/svg+xml"},
	_: {
		client: {start:"_app/immutable/entry/start.B7gMrqR3.js",app:"_app/immutable/entry/app.BqcoySS2.js",imports:["_app/immutable/entry/start.B7gMrqR3.js","_app/immutable/chunks/BN9N5UwC.js","_app/immutable/chunks/CeXHuIk-.js","_app/immutable/chunks/Z7vVkp9t.js","_app/immutable/entry/app.BqcoySS2.js","_app/immutable/chunks/CeXHuIk-.js","_app/immutable/chunks/BZ0qNjTK.js","_app/immutable/chunks/j54S5_w2.js","_app/immutable/chunks/Z7vVkp9t.js","_app/immutable/chunks/h2MRj19v.js","_app/immutable/chunks/CRY3Ohj6.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
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
