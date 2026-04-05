export const manifest = (() => {
function __memo(fn) {
	let value;
	return () => value ??= (value = fn());
}

return {
	appDir: "_app",
	appPath: "_app",
	assets: new Set([".nojekyll","404.html","apps.json","CNAME","favicon.svg","icons/calculator.svg","icons/color-calculator.svg","icons/temp-converter.svg","screenshots/calculator/screenshot.PNG","screenshots/calculator/screenshot2.PNG"]),
	mimeTypes: {".html":"text/html",".json":"application/json",".svg":"image/svg+xml",".PNG":"image/png"},
	_: {
		client: {start:"_app/immutable/entry/start.ByRSmRrZ.js",app:"_app/immutable/entry/app.7Kd4JIbw.js",imports:["_app/immutable/entry/start.ByRSmRrZ.js","_app/immutable/chunks/z4_fwSyX.js","_app/immutable/chunks/CG0alVau.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/BUApaBEI.js","_app/immutable/chunks/DdDE79wW.js","_app/immutable/entry/app.7Kd4JIbw.js","_app/immutable/chunks/CG0alVau.js","_app/immutable/chunks/DIeogL5L.js","_app/immutable/chunks/o-iWUkey.js","_app/immutable/chunks/DcHa5gG-.js","_app/immutable/chunks/DsnmJJEf.js","_app/immutable/chunks/DdDE79wW.js","_app/immutable/chunks/IjSwQgmF.js","_app/immutable/chunks/Bwj5OFLd.js"],stylesheets:[],fonts:[],uses_env_dynamic_public:false},
		nodes: [
			__memo(() => import('./nodes/0.js')),
			__memo(() => import('./nodes/1.js')),
			__memo(() => import('./nodes/2.js')),
			__memo(() => import('./nodes/3.js')),
			__memo(() => import('./nodes/4.js')),
			__memo(() => import('./nodes/5.js'))
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
			},
			{
				id: "/apps",
				pattern: /^\/apps\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 5 },
				endpoint: null
			},
			{
				id: "/app",
				pattern: /^\/app\/?$/,
				params: [],
				page: { layouts: [0,], errors: [1,], leaf: 3 },
				endpoint: null
			},
			{
				id: "/app/[id]",
				pattern: /^\/app\/([^/]+?)\/?$/,
				params: [{"name":"id","optional":false,"rest":false,"chained":false}],
				page: { layouts: [0,], errors: [1,], leaf: 4 },
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
