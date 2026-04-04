
// this file is generated — do not edit it


declare module "svelte/elements" {
	export interface HTMLAttributes<T> {
		'data-sveltekit-keepfocus'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-noscroll'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-preload-code'?:
			| true
			| ''
			| 'eager'
			| 'viewport'
			| 'hover'
			| 'tap'
			| 'off'
			| undefined
			| null;
		'data-sveltekit-preload-data'?: true | '' | 'hover' | 'tap' | 'off' | undefined | null;
		'data-sveltekit-reload'?: true | '' | 'off' | undefined | null;
		'data-sveltekit-replacestate'?: true | '' | 'off' | undefined | null;
	}
}

export {};


declare module "$app/types" {
	type MatcherParam<M> = M extends (param : string) => param is (infer U extends string) ? U : string;

	export interface AppTypes {
		RouteId(): "/" | "/apps" | "/app" | "/app/[id]";
		RouteParams(): {
			"/app/[id]": { id: string }
		};
		LayoutParams(): {
			"/": { id?: string };
			"/apps": Record<string, never>;
			"/app": { id?: string };
			"/app/[id]": { id: string }
		};
		Pathname(): "/" | "/apps" | "/app" | `/app/${string}` & {};
		ResolvedPathname(): `${"" | `/${string}`}${ReturnType<AppTypes['Pathname']>}`;
		Asset(): "/.nojekyll" | "/404.html" | "/apps.json" | "/CNAME" | "/favicon.svg" | "/icons/calculator.svg" | "/icons/color-calculator.svg" | "/icons/temp-converter.svg" | "/screenshots/calculator/screenshot.PNG" | "/screenshots/calculator/screenshot2.PNG" | string & {};
	}
}