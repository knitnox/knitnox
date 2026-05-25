<script lang="ts">
	import { Wrench, Shield, Zap, Cpu } from '@lucide/svelte';
</script>

<div class="flex flex-col items-center justify-center min-h-[85vh] px-4 py-12 transition-colors duration-300">
	<!-- Logo Section -->
	<div class="mb-12">
		<h1 class="logo-title">
			{#each 'Knitnox'.split('') as char}
				<span>{char}</span>
			{/each}
		</h1>
	</div>

	<!-- Hero Text -->
	<div class="max-w-3xl text-center space-y-6 mb-16">
		<h2 class="text-3xl sm:text-5xl font-bold tracking-tight text-zinc-900 dark:text-white">
			The Private, Tool-Enabled <br class="hidden sm:block" />
			<span class="text-blue-600 dark:text-blue-500">Browser-Based</span> LLM Interface
		</h2>
		<p class="text-lg sm:text-xl text-zinc-600 dark:text-zinc-400 leading-relaxed">
			Bring your own API and experience first-class support for MCP (Model Context Protocol). 
			Completely local, high-performance, and designed for privacy.
		</p>
	</div>

	<!-- Features Grid -->
	<div class="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl w-full">
		<div class="feature-card">
			<div class="icon-box text-blue-600 bg-blue-50 dark:bg-blue-900/20">
				<Shield size={24} />
			</div>
			<h3>Privacy First</h3>
			<p>All chat history is stored locally in your browser via IndexedDB. Your keys and data stay on your machine.</p>
		</div>

		<div class="feature-card">
			<div class="icon-box text-orange-500 bg-orange-50 dark:bg-orange-900/20">
				<Wrench size={24} />
			</div>
			<h3>MCP Integrated</h3>
			<p>Connect to any Model Context Protocol server. Dynamically discover and execute tools directly from the chat.</p>
		</div>

		<div class="feature-card">
			<div class="icon-box text-purple-600 bg-purple-50 dark:bg-purple-900/20">
				<Zap size={24} />
			</div>
			<h3>Rich Reasoning</h3>
			<p>Native support for reasoning tokens. Watch the model's internal monologue in expandable "Thought" blocks.</p>
		</div>
	</div>
</div>

<style>
	:root {
		--logo-text: #09090b; /* zinc-950 */
		--logo-glow: rgba(255, 255, 255, 0.8);
		--logo-accent: #ffa500; /* orange */
		--logo-accent-glow: rgba(255, 165, 0, 0.4);
	}

	:global(.dark) {
		--logo-text: #ffffff; /* pure white */
		--logo-glow: rgba(255, 255, 255, 0.1);
		--logo-accent: #ffa500; /* orange */
		--logo-accent-glow: rgba(255, 165, 0, 0.5);
	}

	.logo-title {
		margin: 0;
		font-size: 4rem;
		font-family: 'Orbitron', sans-serif;
		font-weight: 900;
		letter-spacing: 8px;
		color: var(--logo-text);
		display: flex;
		justify-content: center;
		transition: color 0.3s;
		text-shadow: 0 0 20px var(--logo-glow), 0 0 40px var(--logo-glow);
	}

	.logo-title span {
		display: inline-block;
	}

	.logo-title span:nth-child(6) {
		animation: bulbGlow 2s ease-in-out infinite;
		color: var(--logo-accent);
	}

	@keyframes bulbGlow {
		0%, 100% { 
			text-shadow: 0 0 10px var(--logo-accent-glow),
						 0 0 20px rgba(255, 165, 0, 0.3),
						 0 0 30px rgba(255, 165, 0, 0.2);
		}
		50% { 
			text-shadow: 0 0 15px var(--logo-accent-glow),
						 0 0 25px rgba(255, 165, 0, 0.6),
						 0 0 40px rgba(255, 165, 0, 0.4),
						 0 0 60px rgba(255, 165, 0, 0.2);
		}
	}

	.feature-card {
		padding: 24px;
		border-radius: 20px;
		background: transparent;
		border: 1px solid rgba(255, 255, 255, 0.8);
		transition: all 0.3s ease;
	}

	:global(.dark) .feature-card {
		background: transparent;
		border: 1px solid rgba(255, 255, 255, 0.2);
	}

	.feature-card:hover {
		transform: translateY(-4px);
		background: rgba(255, 255, 255, 0.3);
		box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.05);
	}

	:global(.dark) .feature-card:hover {
		background: rgba(255, 255, 255, 0.05);
		box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.3);
	}

	.icon-box {
		width: 48px;
		height: 48px;
		border-radius: 12px;
		display: flex;
		align-items: center;
		justify-content: center;
		margin-bottom: 16px;
	}

	.feature-card h3 {
		font-size: 1.25rem;
		font-weight: 700;
		margin-bottom: 8px;
		color: white;
	}

	.feature-card p {
		font-size: 0.95rem;
		line-height: 1.6;
		color: rgba(255, 255, 255, 0.9);
	}

	/* Removed theme-specific overrides for card text to keep it white */

	@media (max-width: 640px) {
		.logo-title {
			font-size: 2.5rem;
			letter-spacing: 4px;
		}
	}
</style>