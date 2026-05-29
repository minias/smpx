<script>
	import { PlayYoutube, Stop } from "../wailsjs/go/main/App";

	let url = "";
	let loading = false;
	let error = "";

	async function play() {
		error = "";
		loading = true;

		try {
			await PlayYoutube(url);
		} catch (e) {
			error = String(e);
		} finally {
			loading = false;
		}
	}

	async function stop() {
		try {
			await Stop();
		} catch (e) {
			error = String(e);
		}
	}
</script>

<main>
	<h1>SMPX</h1>

	<input
		type="text"
		bind:value={url}
		placeholder="Youtube URL"
	/>

	<div>
		<button on:click={play} disabled={loading}>
			Play
		</button>

		<button on:click={stop}>
			Stop
		</button>
	</div>

	{#if error}
		<pre>{error}</pre>
	{/if}
</main>

<style>
	main {
		padding: 40px;
		font-family: sans-serif;
	}

	input {
		width: 100%;
		padding: 12px;
		margin-bottom: 12px;
	}

	button {
		margin-right: 8px;
		padding: 10px 16px;
	}
</style>