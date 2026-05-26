<script lang="ts">
</script>

<div class="flex flex-col items-center justify-center min-h-[70vh] px-4 py-12 transition-colors duration-300">
	<!-- Logo Section -->
	<div class="mb-12">
		<h1 class="logo-title">
			{#each 'Knitnox'.split('') as char}
				<span>{char}</span>
			{/each}
		</h1>
	</div>

	<!-- Hero Text -->
	<div class="max-w-4xl text-center space-y-6">
		<h2 class="text-2xl sm:text-4xl font-bold tracking-tight text-zinc-900 dark:text-white leading-tight px-2">
			Chat with any AI using any <br class="hidden sm:block" />
			<span class="text-blue-600 dark:text-blue-500">OpenAI-compatible API.</span>
		</h2>
		<p class="text-lg sm:text-xl text-zinc-600 dark:text-zinc-400 leading-relaxed max-w-2xl mx-auto">
			You're in control, private, and never locked in. Feel free to <a href="https://github.com/knitnox/knitnox" target="_blank" rel="noopener noreferrer" class="text-blue-600 dark:text-blue-500 hover:underline">download the source</a> to customize, run locally, or host it yourself on any CDN platform like GitHub Pages.
		</p>
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

	@media (max-width: 640px) {
		.logo-title {
			font-size: 2.5rem;
			letter-spacing: 4px;
		}
	}
</style>
