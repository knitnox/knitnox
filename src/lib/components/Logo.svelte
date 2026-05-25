<script lang="ts">
</script>

<div class="flex flex-col items-center justify-center min-h-[60vh] px-4 transition-colors duration-300">
	<div class="logo-card">
		<h1>
			{#each 'Knitnox'.split('') as char}
				<span>{char}</span>
			{/each}
		</h1>
		<p class="tagline">Your browser is the controller.</p>
		<p class="sub-tagline">Hardware control & privacy-first tools — zero cloud, zero latency.</p>
	</div>
</div>

<style>
	:root {
		--logo-bg: #e6e9ef;
		--logo-shadow-dark: #c2c8d0;
		--logo-shadow-light: #ffffff;
		--logo-text: #2a2f3a;
		--logo-subtext: #666;
	}

	:global(.dark) {
		--logo-bg: #18181b; /* zinc-900 */
		--logo-shadow-dark: #000000;
		--logo-shadow-light: #27272a; /* zinc-800 */
		--logo-text: #f4f4f5; /* zinc-100 */
		--logo-subtext: #a1a1aa; /* zinc-400 */
	}

	.logo-card {
		padding: 24px 32px;
		border-radius: 24px;
		background: var(--logo-bg);
		box-shadow: 8px 8px 16px var(--logo-shadow-dark),
					-8px -8px 16px var(--logo-shadow-light);
		text-align: center;
		position: relative;
		animation: cardGlowRandom 60s infinite ease-in-out;
		max-width: 500px;
		transition: background-color 0.3s, box-shadow 0.3s;
	}

	@keyframes cardGlowRandom {
		0%, 100% {
			box-shadow: 8px 8px 16px var(--logo-shadow-dark),
						-8px -8px 16px var(--logo-shadow-light),
						0 0 15px rgba(255, 0, 150, 0.2);
		}
		20% {
			box-shadow: 8px 8px 16px var(--logo-shadow-dark),
						-8px -8px 16px var(--logo-shadow-light),
						0 0 20px rgba(0, 255, 200, 0.25);
		}
		40% {
			box-shadow: 8px 8px 16px var(--logo-shadow-dark),
						-8px -8px 16px var(--logo-shadow-light),
						0 0 18px rgba(100, 150, 255, 0.2);
		}
		60% {
			box-shadow: 8px 8px 16px var(--logo-shadow-dark),
						-8px -8px 16px var(--logo-shadow-light),
						0 0 22px rgba(255, 200, 0, 0.25);
		}
		80% {
			box-shadow: 8px 8px 16px var(--logo-shadow-dark),
						-8px -8px 16px var(--logo-shadow-light),
						0 0 16px rgba(150, 0, 255, 0.2);
		}
	}

	h1 {
		margin: 0;
		font-size: 3.5rem;
		font-family: 'Orbitron', sans-serif;
		font-weight: 900;
		letter-spacing: 6px;
		color: var(--logo-text);
		display: flex;
		justify-content: center;
		transition: color 0.3s;
	}

	h1 span {
		display: inline-block;
	}

	h1 span:nth-child(6) {
		animation: bulbGlow 2s ease-in-out infinite;
		color: #ffa500;
	}

	@keyframes bulbGlow {
		0%, 100% { 
			text-shadow: 0 0 10px rgba(255, 200, 0, 0.4),
						 0 0 20px rgba(255, 165, 0, 0.5),
						 0 0 30px rgba(255, 140, 0, 0.3);
		}
		50% { 
			text-shadow: 0 0 20px rgba(255, 200, 0, 0.8),
						 0 0 35px rgba(255, 165, 0, 0.9),
						 0 0 50px rgba(255, 140, 0, 0.6),
						 0 0 70px rgba(255, 120, 0, 0.4);
		}
	}

	.tagline {
		margin: 20px 0 8px 0;
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--logo-text);
		letter-spacing: 0.5px;
		text-align: center;
		transition: color 0.3s;
	}

	.sub-tagline {
		margin: 8px 0 0 0;
		font-size: 0.95rem;
		color: var(--logo-subtext);
		line-height: 1.55;
		text-align: center;
		transition: color 0.3s;
	}

	@media (max-width: 640px) {
		h1 {
			font-size: 2.5rem;
			letter-spacing: 4px;
		}
		.logo-card {
			padding: 20px 24px;
		}
	}
</style>