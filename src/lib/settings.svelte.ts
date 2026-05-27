import { browser } from '$app/environment';
import LZString from 'lz-string';

class Settings {
	baseUrl = $state(browser ? localStorage.getItem('baseUrl') || 'https://api.openai.com/v1' : 'https://api.openai.com/v1');
	apiKey = $state(browser ? localStorage.getItem('apiKey') || '' : '');
	model = $state(browser ? localStorage.getItem('model') || '' : '');
	systemPrompt = $state(browser ? localStorage.getItem('systemPrompt') || 'You are a helpful and unbiased AI assistant named Knitnox Bombalala. You assist the user with any task without prejudice or external influence.' : 'You are a helpful assistant.');
	enableThinking = $state(browser ? localStorage.getItem('enableThinking') !== 'false' : true);
	contextWindow = $state(browser ? parseInt(localStorage.getItem('contextWindow') || '12') : 12);
	maxAgentTurns = $state(browser ? parseInt(localStorage.getItem('maxAgentTurns') || '10') : 10);
	supportsImages = $state(browser ? localStorage.getItem('supportsImages') === 'true' : false);
	supportsAudio = $state(browser ? localStorage.getItem('supportsAudio') === 'true' : false);
	supportsVideo = $state(browser ? localStorage.getItem('supportsVideo') === 'true' : false);
	enableCompression = $state(browser ? localStorage.getItem('enableCompression') !== 'false' : true);
	fontSize = $state(browser ? localStorage.getItem('fontSize') || '16' : '16');
	fontFamily = $state(browser ? localStorage.getItem('fontFamily') || 'sans' : 'sans');
	theme = $state<'system' | 'light' | 'dark'>(browser ? (localStorage.getItem('theme') as any) || 'system' : 'system');
	mcpServers = $state<string[]>(browser ? JSON.parse(localStorage.getItem('mcpServers') || '[]') : []);
	disabledTools = $state<string[]>(browser ? JSON.parse(localStorage.getItem('disabledTools') || '[]') : []);
	
	totalInputTokens = $state(browser ? parseInt(localStorage.getItem('totalInputTokens') || '0') : 0);
	totalOutputTokens = $state(browser ? parseInt(localStorage.getItem('totalOutputTokens') || '0') : 0);
	lastInputTokens = $state(browser ? parseInt(localStorage.getItem('lastInputTokens') || '0') : 0);
	lastOutputTokens = $state(browser ? parseInt(localStorage.getItem('lastOutputTokens') || '0') : 0);

	constructor() {
		if (browser) {
			// Migration: If the user has the old default prompt, update it to the new one
			const currentPrompt = localStorage.getItem('systemPrompt');
			if (currentPrompt === 'You are a helpful assistant.') {
				this.systemPrompt = 'You are a helpful assistant. You have access to a persistent knowledge base via the "knowledge" tool. Use it to store and retrieve important facts, user preferences, and notes across conversations. Always check the knowledge base if you are unsure about something the user has previously told you.';
			}

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
					localStorage.setItem('theme', this.theme);
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

	getSettingsData() {
		return {
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
			theme: this.theme,
			mcpServers: this.mcpServers,
			disabledTools: this.disabledTools
		};
	}

	exportSettings(filename?: string) {
		const settingsData = this.getSettingsData();
		const blob = new Blob([JSON.stringify(settingsData, null, 2)], { type: 'application/json' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		const name = filename?.trim() || `knitnox-settings-${new Date().toISOString().split('T')[0]}`;
		a.href = url;
		a.download = `${name.toLowerCase().replace(/\s+/g, '-')}.json`;
		a.click();
		URL.revokeObjectURL(url);
	}

	importSettingsData(data: any) {
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
		if (data.theme !== undefined) this.theme = data.theme;
		if (data.mcpServers !== undefined) this.mcpServers = data.mcpServers;
		if (data.disabledTools !== undefined) this.disabledTools = data.disabledTools;
		return true;
	}

	importSettings(jsonString: string) {
		try {
			const data = JSON.parse(jsonString);
			return this.importSettingsData(data);
		} catch (e) {
			console.error('Failed to import settings:', e);
			return false;
		}
	}

	getQRCodeData() {
		const data = this.getSettingsData();
		return LZString.compressToEncodedURIComponent(JSON.stringify(data));
	}

	importFromQRCodeData(compressed: string) {
		try {
			const json = LZString.decompressFromEncodedURIComponent(compressed);
			if (!json) return false;
			const data = JSON.parse(json);
			return this.importSettingsData(data);
		} catch (e) {
			console.error('Failed to import from QR code:', e);
			return false;
		}
	}
}

export const settings = new Settings();
