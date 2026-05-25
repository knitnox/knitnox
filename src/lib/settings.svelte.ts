import { browser } from '$app/environment';

class Settings {
	baseUrl = $state(browser ? localStorage.getItem('baseUrl') || 'https://api.openai.com/v1' : 'https://api.openai.com/v1');
	apiKey = $state(browser ? localStorage.getItem('apiKey') || '' : '');
	model = $state(browser ? localStorage.getItem('model') || 'gpt-4o' : 'gpt-4o');
	systemPrompt = $state(browser ? localStorage.getItem('systemPrompt') || 'You are a helpful assistant.' : 'You are a helpful assistant.');
	enableThinking = $state(browser ? localStorage.getItem('enableThinking') !== 'false' : true);
	contextWindow = $state(browser ? parseInt(localStorage.getItem('contextWindow') || '20') : 20);
	supportsImages = $state(browser ? localStorage.getItem('supportsImages') === 'true' : false);
	supportsAudio = $state(browser ? localStorage.getItem('supportsAudio') === 'true' : false);
	supportsVideo = $state(browser ? localStorage.getItem('supportsVideo') === 'true' : false);
	fontSize = $state(browser ? localStorage.getItem('fontSize') || '16' : '16');
	fontFamily = $state(browser ? localStorage.getItem('fontFamily') || 'sans' : 'sans');
	mcpServers = $state<string[]>(browser ? JSON.parse(localStorage.getItem('mcpServers') || '[]') : []);
	
	totalInputTokens = $state(browser ? parseInt(localStorage.getItem('totalInputTokens') || '0') : 0);
	totalOutputTokens = $state(browser ? parseInt(localStorage.getItem('totalOutputTokens') || '0') : 0);
	lastInputTokens = $state(browser ? parseInt(localStorage.getItem('lastInputTokens') || '0') : 0);
	lastOutputTokens = $state(browser ? parseInt(localStorage.getItem('lastOutputTokens') || '0') : 0);

	constructor() {
		if (browser) {
			$effect.root(() => {
				$effect(() => {
					localStorage.setItem('baseUrl', this.baseUrl);
				});
				$effect(() => {
					localStorage.setItem('apiKey', this.apiKey);
				});
				$effect(() => {
					localStorage.setItem('model', this.model);
				});
				$effect(() => {
					localStorage.setItem('systemPrompt', this.systemPrompt);
				});
				$effect(() => {
					localStorage.setItem('enableThinking', String(this.enableThinking));
				});
				$effect(() => {
					localStorage.setItem('contextWindow', String(this.contextWindow));
				});
				$effect(() => {
					localStorage.setItem('supportsImages', String(this.supportsImages));
				});
				$effect(() => {
					localStorage.setItem('supportsAudio', String(this.supportsAudio));
				});
				$effect(() => {
					localStorage.setItem('supportsVideo', String(this.supportsVideo));
				});
				$effect(() => {
					localStorage.setItem('fontSize', this.fontSize);
				});
				$effect(() => {
					localStorage.setItem('fontFamily', this.fontFamily);
				});
				$effect(() => {
					localStorage.setItem('mcpServers', JSON.stringify(this.mcpServers));
				});
				$effect(() => {
					localStorage.setItem('totalInputTokens', String(this.totalInputTokens));
				});
				$effect(() => {
					localStorage.setItem('totalOutputTokens', String(this.totalOutputTokens));
				});
				$effect(() => {
					localStorage.setItem('lastInputTokens', String(this.lastInputTokens));
				});
				$effect(() => {
					localStorage.setItem('lastOutputTokens', String(this.lastOutputTokens));
				});
			});
		}
	}
}

export const settings = new Settings();
