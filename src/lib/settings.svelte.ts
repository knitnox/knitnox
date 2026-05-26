import { browser } from '$app/environment';

class Settings {
	baseUrl = $state(browser ? localStorage.getItem('baseUrl') || 'https://api.openai.com/v1' : 'https://api.openai.com/v1');
	apiKey = $state(browser ? localStorage.getItem('apiKey') || '' : '');
	model = $state(browser ? localStorage.getItem('model') || 'gpt-4o' : 'gpt-4o');
	systemPrompt = $state(browser ? localStorage.getItem('systemPrompt') || 'You are a helpful assistant.' : 'You are a helpful assistant.');
	enableThinking = $state(browser ? localStorage.getItem('enableThinking') !== 'false' : true);
	contextWindow = $state(browser ? parseInt(localStorage.getItem('contextWindow') || '12') : 12);
	maxAgentTurns = $state(browser ? parseInt(localStorage.getItem('maxAgentTurns') || '10') : 10);
	supportsImages = $state(browser ? localStorage.getItem('supportsImages') === 'true' : false);
	supportsAudio = $state(browser ? localStorage.getItem('supportsAudio') === 'true' : false);
	supportsVideo = $state(browser ? localStorage.getItem('supportsVideo') === 'true' : false);
	enableCompression = $state(browser ? localStorage.getItem('enableCompression') !== 'false' : true);
	fontSize = $state(browser ? localStorage.getItem('fontSize') || '16' : '16');
	fontFamily = $state(browser ? localStorage.getItem('fontFamily') || 'sans' : 'sans');
	mcpServers = $state<string[]>(browser ? JSON.parse(localStorage.getItem('mcpServers') || '[]') : []);
	disabledTools = $state<string[]>(browser ? JSON.parse(localStorage.getItem('disabledTools') || '[]') : []);
	
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
					localStorage.setItem('maxAgentTurns', String(this.maxAgentTurns));
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
					localStorage.setItem('enableCompression', String(this.enableCompression));
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
					localStorage.setItem('disabledTools', JSON.stringify(this.disabledTools));
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

	exportSettings() {
		const settingsData = {
			baseUrl: this.baseUrl,
			apiKey: this.apiKey,
			model: this.model,
			systemPrompt: this.systemPrompt,
			enableThinking: this.enableThinking,
			contextWindow: this.contextWindow,
			maxAgentTurns: this.maxAgentTurns,
			supportsImages: this.supportsImages,
			supportsAudio: this.supportsAudio,
			supportsVideo: this.supportsVideo,
			enableCompression: this.enableCompression,
			fontSize: this.fontSize,
			fontFamily: this.fontFamily,
			mcpServers: this.mcpServers,
			disabledTools: this.disabledTools
		};
		const blob = new Blob([JSON.stringify(settingsData, null, 2)], { type: 'application/json' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `knitnox-settings-${new Date().toISOString().split('T')[0]}.json`;
		a.click();
		URL.revokeObjectURL(url);
	}

	importSettings(jsonString: string) {
		try {
			const data = JSON.parse(jsonString);
			if (data.baseUrl !== undefined) this.baseUrl = data.baseUrl;
			if (data.apiKey !== undefined) this.apiKey = data.apiKey;
			if (data.model !== undefined) this.model = data.model;
			if (data.systemPrompt !== undefined) this.systemPrompt = data.systemPrompt;
			if (data.enableThinking !== undefined) this.enableThinking = data.enableThinking;
			if (data.contextWindow !== undefined) this.contextWindow = data.contextWindow;
			if (data.maxAgentTurns !== undefined) this.maxAgentTurns = data.maxAgentTurns;
			if (data.supportsImages !== undefined) this.supportsImages = data.supportsImages;
			if (data.supportsAudio !== undefined) this.supportsAudio = data.supportsAudio;
			if (data.supportsVideo !== undefined) this.supportsVideo = data.supportsVideo;
			if (data.enableCompression !== undefined) this.enableCompression = data.enableCompression;
			if (data.fontSize !== undefined) this.fontSize = data.fontSize;
			if (data.fontFamily !== undefined) this.fontFamily = data.fontFamily;
			if (data.mcpServers !== undefined) this.mcpServers = data.mcpServers;
			if (data.disabledTools !== undefined) this.disabledTools = data.disabledTools;
			return true;
		} catch (e) {
			console.error('Failed to import settings:', e);
			return false;
		}
	}
}

export const settings = new Settings();
