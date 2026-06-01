import { browser } from '$app/environment';
import LZString from 'lz-string';
import { db, type Profile } from './db';
import { toast } from './toast.svelte';

class Settings {
	// Model-specific settings (now managed via profiles)
	baseUrl = $state('https://api.openai.com/v1');
	apiKey = $state('');
	model = $state('');
	systemPrompt = $state('You are a helpful and unbiased AI assistant named Knitnox Bombalala. You assist the user with any task without prejudice or external influence.');
	enableThinking = $state(true);
	contextWindow = $state(12);
	maxAgentTurns = $state(10);
	supportsImages = $state(false);
	supportsAudio = $state(false);
	supportsVideo = $state(false);
	enableCompression = $state(true);
	mcpServers = $state<string[]>([]);
	disabledTools = $state<string[]>([]);
	temperature = $state<number | undefined>(undefined);
	top_p = $state<number | undefined>(undefined);
	frequency_penalty = $state<number | undefined>(undefined);
	presence_penalty = $state<number | undefined>(undefined);
	response_format = $state<string | undefined>(undefined);
	reasoning_effort = $state<'low' | 'medium' | 'high' | undefined>(undefined);
	seed = $state<number | undefined>(undefined);

	// Profile Management
	profiles = $state<Profile[]>([]);
	activeProfileId = $state<number | null>(null);
	isLoading = $state(true);
	private isSwitchingProfile = false;

	// Global UI settings
	fontSize = $state(browser ? localStorage.getItem('fontSize') || '16' : '16');
	fontFamily = $state(browser ? localStorage.getItem('fontFamily') || 'sans' : 'sans');
	theme = $state<'system' | 'light' | 'dark'>(browser ? (localStorage.getItem('theme') as any) || 'system' : 'system');
	shouldFlashSettings = $state(false);
	shouldFlashWrench = $state(false);
	
	// Global Token stats
	totalInputTokens = $state(browser ? parseInt(localStorage.getItem('totalInputTokens') || '0') : 0);
	totalOutputTokens = $state(browser ? parseInt(localStorage.getItem('totalOutputTokens') || '0') : 0);
	lastInputTokens = $state(browser ? parseInt(localStorage.getItem('lastInputTokens') || '0') : 0);
	lastOutputTokens = $state(browser ? parseInt(localStorage.getItem('lastOutputTokens') || '0') : 0);

	constructor() {
		if (browser) {
			this.init();
			
			$effect.root(() => {
				// Global UI settings effects
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

				// Auto-save active profile whenever its settings change
				$effect(() => {
					// Track dependencies
					const _ = [
						this.activeProfileId, this.baseUrl, this.apiKey, this.model, 
						this.systemPrompt, this.enableThinking, this.contextWindow, 
						this.maxAgentTurns, this.supportsImages, this.supportsAudio, 
						this.supportsVideo, this.enableCompression, this.mcpServers, 
						this.disabledTools, this.temperature, this.top_p, 
						this.frequency_penalty, this.presence_penalty, 
						this.response_format, this.reasoning_effort, this.seed
					];
					
					if (this.activeProfileId !== null && !this.isSwitchingProfile) {
						this.saveActiveProfile();
					}
				});
			});
		}
	}

	async init() {
		await this.loadProfiles();
		
		const savedActiveId = localStorage.getItem('activeProfileId');
		if (savedActiveId && this.profiles.find(p => p.id === parseInt(savedActiveId))) {
			await this.switchProfile(parseInt(savedActiveId));
		} else if (this.profiles.length > 0) {
			await this.switchProfile(this.profiles[0].id!);
		} else {
			// Create default profile from existing localStorage if available
			const defaultProfile: Omit<Profile, 'id'> = {
				name: 'Default Profile',
				baseUrl: localStorage.getItem('baseUrl') || 'https://api.openai.com/v1',
				apiKey: localStorage.getItem('apiKey') || '',
				model: localStorage.getItem('model') || '',
				systemPrompt: localStorage.getItem('systemPrompt') || 'You are a helpful and unbiased AI assistant named Knitnox Bombalala. You assist the user with any task without prejudice or external influence.',
				enableThinking: localStorage.getItem('enableThinking') !== 'false',
				contextWindow: parseInt(localStorage.getItem('contextWindow') || '12'),
				maxAgentTurns: parseInt(localStorage.getItem('maxAgentTurns') || '10'),
				supportsImages: localStorage.getItem('supportsImages') === 'true',
				supportsAudio: localStorage.getItem('supportsAudio') === 'true',
				supportsVideo: localStorage.getItem('supportsVideo') === 'true',
				enableCompression: localStorage.getItem('enableCompression') !== 'false',
				mcpServers: JSON.parse(localStorage.getItem('mcpServers') || '[]'),
				disabledTools: JSON.parse(localStorage.getItem('disabledTools') || '[]'),
				createdAt: Date.now(),
				updatedAt: Date.now()
			};
			const id = await db.profiles.add(defaultProfile as Profile);
			await this.loadProfiles();
			await this.switchProfile(id);
		}
		this.isLoading = false;
	}

	async loadProfiles() {
		this.profiles = await db.profiles.toArray();
	}

	async switchProfile(id: number) {
		const profile = this.profiles.find(p => p.id === id);
		if (profile) {
			this.isSwitchingProfile = true;
			
			this.activeProfileId = id;
			this.baseUrl = profile.baseUrl;
			this.apiKey = profile.apiKey;
			this.model = profile.model;
			this.systemPrompt = profile.systemPrompt;
			this.enableThinking = profile.enableThinking;
			this.contextWindow = profile.contextWindow;
			this.maxAgentTurns = profile.maxAgentTurns;
			this.supportsImages = profile.supportsImages;
			this.supportsAudio = profile.supportsAudio;
			this.supportsVideo = profile.supportsVideo;
			this.enableCompression = profile.enableCompression;
			this.mcpServers = profile.mcpServers;
			this.disabledTools = profile.disabledTools;
			this.temperature = profile.temperature;
			this.top_p = profile.top_p;
			this.frequency_penalty = profile.frequency_penalty;
			this.presence_penalty = profile.presence_penalty;
			this.response_format = profile.response_format;
			this.reasoning_effort = profile.reasoning_effort;
			this.seed = profile.seed;
			
			localStorage.setItem('activeProfileId', String(id));
			
			// Reset flag in next tick to allow auto-save effect to skip this batch
			setTimeout(() => {
				this.isSwitchingProfile = false;
			}, 0);
		}
	}

	async saveActiveProfile() {
		if (this.activeProfileId === null) return;
		
		await db.profiles.update(this.activeProfileId, {
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
			mcpServers: $state.snapshot(this.mcpServers),
			disabledTools: $state.snapshot(this.disabledTools),
			temperature: this.temperature,
			top_p: this.top_p,
			frequency_penalty: this.frequency_penalty,
			presence_penalty: this.presence_penalty,
			response_format: this.response_format,
			reasoning_effort: this.reasoning_effort,
			seed: this.seed,
			updatedAt: Date.now()
		});
		// Update the profile in the local list too to keep it in sync
		const idx = this.profiles.findIndex(p => p.id === this.activeProfileId);
		if (idx !== -1) {
			this.profiles[idx] = { ...this.profiles[idx], 
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
				mcpServers: $state.snapshot(this.mcpServers),
				disabledTools: $state.snapshot(this.disabledTools),
				temperature: this.temperature,
				top_p: this.top_p,
				frequency_penalty: this.frequency_penalty,
				presence_penalty: this.presence_penalty,
				response_format: this.response_format,
				reasoning_effort: this.reasoning_effort,
				seed: this.seed,
				updatedAt: Date.now()
			};
		}
	}

	async createProfile(name: string) {
		const newProfile: Omit<Profile, 'id'> = {
			name,
			baseUrl: 'https://api.openai.com/v1',
			apiKey: '',
			model: '',
			systemPrompt: 'You are a helpful and unbiased AI assistant named Knitnox Bombalala. You assist the user with any task without prejudice or external influence.',
			enableThinking: true,
			contextWindow: 12,
			maxAgentTurns: 10,
			supportsImages: false,
			supportsAudio: false,
			supportsVideo: false,
			enableCompression: true,
			mcpServers: [],
			disabledTools: [],
			createdAt: Date.now(),
			updatedAt: Date.now()
		};
		const id = await db.profiles.add(newProfile as Profile);
		await this.loadProfiles();
		await this.switchProfile(id);
		return id;
	}

	async duplicateProfile(id: number) {
		try {
			const profile = this.profiles.find(p => p.id === id);
			if (!profile) {
				toast.add('Profile not found', 'error');
				return;
			}
			
			// Get clean data from snapshot
			const snapped = $state.snapshot(profile);
			const { id: _, ...profileWithoutId } = snapped;
			
			const newProfile: Omit<Profile, 'id'> = {
				...profileWithoutId,
				name: `${profile.name} (Copy)`,
				createdAt: Date.now(),
				updatedAt: Date.now()
			};

			// If duplicating the active profile, ensure we have the absolute latest live settings
			if (id === this.activeProfileId) {
				Object.assign(newProfile, {
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
					mcpServers: $state.snapshot(this.mcpServers),
					disabledTools: $state.snapshot(this.disabledTools),
					temperature: this.temperature,
					top_p: this.top_p,
					frequency_penalty: this.frequency_penalty,
					presence_penalty: this.presence_penalty,
					response_format: this.response_format,
					reasoning_effort: this.reasoning_effort,
					seed: this.seed
				});
			}

			const newId = await db.profiles.add(newProfile as Profile);
			await this.loadProfiles();
			await this.switchProfile(newId);
			toast.add(`Duplicated profile as "${newProfile.name}"`, 'success');
			return newId;
		} catch (error) {
			console.error('Failed to duplicate profile:', error);
			toast.add('Failed to duplicate profile', 'error');
		}
	}

	async deleteProfile(id: number) {
		if (this.profiles.length <= 1) {
			alert('Cannot delete the last profile.');
			return;
		}
		
		await db.profiles.delete(id);
		await this.loadProfiles();
		
		if (this.activeProfileId === id) {
			await this.switchProfile(this.profiles[0].id!);
		}
	}

	async updateProfileName(id: number, newName: string) {
		await db.profiles.update(id, { name: newName, updatedAt: Date.now() });
		const idx = this.profiles.findIndex(p => p.id === id);
		if (idx !== -1) {
			this.profiles[idx].name = newName;
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
		const data = {
			baseUrl: this.baseUrl,
			apiKey: this.apiKey,
			model: this.model,
			supportsImages: this.supportsImages,
			supportsAudio: this.supportsAudio,
			supportsVideo: this.supportsVideo,
		};
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
