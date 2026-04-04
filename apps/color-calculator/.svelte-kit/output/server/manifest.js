export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "apps/color-calculator/_app",
	assets: new Set([]),
	mimeTypes: {},
	_: {
		client: {start:"_app/immutable/entry/start.CItekVTk.js",app:"_app/immutable/entry/app.CfmrgTCT.js",imports:["_app/immutable/entry/start.CItekVTk.js","_app/immutable/chunks/B9lq-9vX.js","_app/immutable/chunks/B010l9nW.js","_app/immutable/chunks/CGfwebDj.js","_app/immutable/entry/app.CfmrgTCT.js","_app/immutable/chunks/B010l9nW.js","_app/immutable/chunks/CUV0aFCv.js","_app/immutable/chunks/DBM8aOVT.js","_app/immutable/chunks/CGfwebDj.js","_app/immutable/chunks/BRqqvdLY.js","_app/immutable/chunks/CtBTFLmV.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
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
