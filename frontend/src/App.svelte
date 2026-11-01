<script>
	import { onDestroy, onMount } from "svelte";
	import {
		AddTrack,
		AudioEnded,
		AudioFailed,
		AudioPaused,
		AudioProgress,
		CheckForUpdate,
		CycleLoopMode,
		GetState,
		Next,
		OpenURL,
		Play,
		PlayPause,
		Previous,
		RemoveTrack,
		SetShowVideo,
		SetVolume,
		Stop,
		Version,
	} from "../wailsjs/go/main/App";
	import { EventsOff, EventsOn } from "../wailsjs/runtime/runtime";

	const STATE_LABELS = {
		playing: "재생중",
		loading: "로딩중",
		paused: "일시정지",
		stopped: "정지",
	};

	const LOOP_LABELS = {
		none: "🔁 1회",
		all: "🔁 전체",
		single: "🔂 1곡",
	};

	let player = {
		playlist: [],
		currentIndex: -1,
		state: "stopped",
		loopMode: "none",
		volume: 50,
		showVideo: false,
	};

	let url = "";
	let adding = false;
	let selected = -1;
	let error = "";
	let notice = "";
	let noticeTimer = null;
	let updateInfo = null;
	let version = "";

	// yt-dlp 준비 상태: preparing | ready | error
	let ytdlpStatus = "";

	// 실제 재생은 <audio>, 재생 상태는 Go(PlayerService)가 관리
	let audio;
	let audioSeq = 0; // 0 = 정지 (이전 곡 이벤트 무시)
	let lastProgress = 0;

	// 슬라이더 조작 중(디바운스 대기)에는 서버 상태로 덮어쓰지 않음
	let volume = 50;
	let volumeTimer = null;

	$: current =
		player.currentIndex >= 0 && player.currentIndex < player.playlist.length
			? player.playlist[player.currentIndex]
			: null;
	$: isPlaying = player.state === "playing";
	$: isLoading = player.state === "loading";
	$: statusText = STATE_LABELS[player.state] ?? player.state;

	function applyState(s) {
		if (!s) return;
		player = { ...s, playlist: s.playlist ?? [] };
		if (volumeTimer === null) {
			volume = player.volume;
		}
		if (selected >= player.playlist.length) {
			selected = player.playlist.length - 1;
		}
	}

	function showNotice(text) {
		notice = text;
		clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = ""), 4000);
	}

	function showError(e) {
		error = typeof e === "string" ? e : e?.message ?? String(e);
	}

	async function run(fn) {
		try {
			await fn();
		} catch (e) {
			showError(e);
		}
	}

	async function addTrack() {
		const value = url.trim();
		if (!value || adding) return;
		adding = true;
		error = "";
		try {
			const res = await AddTrack(value);
			url = "";
			if (res && (res.added > 1 || res.skipped > 0)) {
				showNotice(`${res.added}곡 추가` + (res.skipped ? ` (중복 ${res.skipped}곡 제외)` : ""));
			}
		} catch (e) {
			showError(e);
		} finally {
			adding = false;
		}
	}

	function onInputKey(e) {
		if (e.key === "Enter") {
			e.preventDefault();
			addTrack();
		}
	}

	function removeTrack(index) {
		if (index < 0) return;
		run(() => RemoveTrack(index));
	}

	function playIndex(index) {
		selected = index;
		run(() => Play(index));
	}

	function togglePlay() {
		// 정지 상태에서 선택된 곡이 있으면 그 곡부터 재생
		if (player.state === "stopped" && selected >= 0 && selected !== player.currentIndex) {
			playIndex(selected);
			return;
		}
		run(PlayPause);
	}

	const stop = () => run(Stop);
	const next = () => run(Next);
	const previous = () => run(Previous);
	const cycleLoop = () => run(CycleLoopMode);

	function setVolume(v) {
		volume = Math.max(0, Math.min(100, Math.round(v)));
		clearTimeout(volumeTimer);
		clearTimeout(noticeTimer);
		volumeTimer = setTimeout(async () => {
			volumeTimer = null;
			await run(() => SetVolume(volume));
		}, 100);
	}

	function isTextInput(el) {
		if (!el) return false;
		if (el.tagName === "INPUT") return el.type !== "range";
		return el.tagName === "TEXTAREA" || el.isContentEditable;
	}

	function onGlobalKey(e) {
		if (isTextInput(document.activeElement)) return;
		if (e.altKey || e.ctrlKey || e.metaKey) return;

		// 포커스된 버튼이 Space 로 한 번 더 클릭되는 것 방지
		if (e.key === " " && document.activeElement?.tagName === "BUTTON") {
			document.activeElement.blur();
		}

		// 한글 입력 상태에서도 동작하도록 물리 키(code)로 판별
		if (e.code === "KeyV") {
			e.preventDefault();
			toggleVideo();
			return;
		}

		switch (e.key) {
			case " ":
				e.preventDefault();
				togglePlay();
				break;
			case "ArrowLeft":
				e.preventDefault();
				previous();
				break;
			case "ArrowRight":
				e.preventDefault();
				next();
				break;
			case "ArrowUp":
				e.preventDefault();
				setVolume(volume + 5);
				break;
			case "ArrowDown":
				e.preventDefault();
				setVolume(volume - 5);
				break;
			case "Delete":
				if (selected >= 0) {
					e.preventDefault();
					removeTrack(selected);
				}
				break;
		}
	}

	// ---- Go → <audio> ----

	function playAudio(seq) {
		audio.play().catch((e) => {
			// 다음 곡 로드로 중단된 경우는 정상
			if (e?.name !== "AbortError" && seq === audioSeq) {
				AudioFailed(seq, String(e?.message ?? e));
			}
		});
	}

	function onAudioCommand(cmd) {
		if (!audio || !cmd) return;

		switch (cmd.action) {
			case "play":
				audioSeq = cmd.seq;
				audio.src = cmd.url;
				audio.volume = cmd.volume / 100;
				if (cmd.startAt > 0) audio.currentTime = cmd.startAt;
				lastProgress = cmd.startAt || 0;
				playAudio(cmd.seq);
				break;
			case "pause":
				audio.pause();
				break;
			case "resume":
				playAudio(audioSeq);
				break;
			case "stop":
				audioSeq = 0;
				audio.pause();
				audio.removeAttribute("src");
				audio.load();
				break;
			case "volume":
				audio.volume = cmd.volume / 100;
				break;
		}
	}

	// ---- <audio> → Go ----

	const MEDIA_ERRORS = {
		1: "재생이 중단되었습니다",
		2: "네트워크 오류",
		3: "오디오 디코딩 실패",
		4: "지원하지 않는 형식이거나 스트림을 열 수 없습니다",
	};

	function onAudioEnded() {
		if (audioSeq) AudioEnded(audioSeq);
	}

	function onAudioError() {
		if (!audioSeq) return;
		const code = audio.error?.code;
		AudioFailed(audioSeq, MEDIA_ERRORS[code] ?? `오디오 오류 (${code})`);
	}

	function onAudioPause() {
		if (audioSeq && !audio.ended) AudioPaused(audioSeq, true);
	}

	function onAudioPlaying() {
		if (audioSeq) AudioPaused(audioSeq, false);
	}

	// 1초마다 재생 위치 통보 (화면 보기 전환 시 이어서 재생)
	function onAudioTimeUpdate() {
		if (audioSeq && Math.abs(audio.currentTime - lastProgress) >= 1) {
			lastProgress = audio.currentTime;
			AudioProgress(audioSeq, audio.currentTime);
		}
	}

	const toggleVideo = () => run(() => SetShowVideo(!player.showVideo));

	// ---- MediaSession: macOS '지금 재생 중', 미디어 키, AirPods ----

	function setupMediaSession() {
		if (!("mediaSession" in navigator)) return;
		const handlers = {
			play: () => player.state !== "playing" && togglePlay(),
			pause: () => player.state === "playing" && run(PlayPause),
			stop,
			nexttrack: next,
			previoustrack: previous,
		};
		for (const [action, fn] of Object.entries(handlers)) {
			try {
				navigator.mediaSession.setActionHandler(action, fn);
			} catch {
				// 지원하지 않는 액션
			}
		}
	}

	$: if (typeof navigator !== "undefined" && "mediaSession" in navigator) {
		const ms = navigator.mediaSession;
		ms.playbackState = isPlaying ? "playing" : player.state === "paused" ? "paused" : "none";
		ms.metadata = current ? new MediaMetadata({ title: current.title, artist: "SMPX" }) : null;
	}

	function marker(index, currentIndex, state) {
		if (index !== currentIndex) return "";
		if (state === "playing") return "▶";
		if (state === "paused") return "⏸";
		if (state === "loading") return "⏳";
		return "";
	}

	onMount(async () => {
		EventsOn("player:state", applyState);
		EventsOn("player:error", showError);
		EventsOn("audio:command", onAudioCommand);
		EventsOn("ytdlp:status", (s) => (ytdlpStatus = s));
		setupMediaSession();

		try {
			applyState(await GetState());
		} catch (e) {
			showError(e);
		}

		Version()
			.then((v) => (version = v))
			.catch(() => {});

		CheckForUpdate()
			.then((info) => (updateInfo = info || null))
			.catch(() => {});
	});

	onDestroy(() => {
		EventsOff("player:state", "player:error", "audio:command", "ytdlp:status");
		clearTimeout(volumeTimer);
	});
</script>

<svelte:window on:keydown={onGlobalKey} />

<audio
	bind:this={audio}
	preload="auto"
	on:ended={onAudioEnded}
	on:error={onAudioError}
	on:pause={onAudioPause}
	on:playing={onAudioPlaying}
	on:timeupdate={onAudioTimeUpdate}
></audio>

<main>
	{#if ytdlpStatus === "preparing"}
		<div class="banner">
			<span class="spinner"></span>
			<span>yt-dlp 준비 중… (최초 실행 시 다운로드)</span>
		</div>
	{/if}

	{#if updateInfo}
		<div class="banner">
			<span>새 버전 <b>{updateInfo.version}</b> 이(가) 있습니다.</span>
			<button class="small" on:click={() => OpenURL(updateInfo.url)}>다운로드</button>
			<button class="icon" title="닫기" on:click={() => (updateInfo = null)}>✕</button>
		</div>
	{/if}

	<div class="add-row">
		<input
			type="text"
			bind:value={url}
			on:keydown={onInputKey}
			placeholder="YouTube 영상 또는 재생목록 URL 입력 후 Enter"
			disabled={adding}
			spellcheck="false"
		/>
		<button class="primary" on:click={addTrack} disabled={adding || !url.trim()}>
			{#if adding}
				<span class="spinner"></span>
				조회중
			{:else}
				➕ 추가
			{/if}
		</button>
	</div>

	<ul class="playlist">
		{#each player.playlist as item, i (item.url)}
			<!-- 키보드 조작은 전역 단축키(Space/←/→/Delete)로 처리 -->
			<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
			<li
				class:selected={i === selected}
				class:current={i === player.currentIndex}
				title={item.url}
				on:click={() => (selected = i)}
				on:dblclick={() => playIndex(i)}
			>
				<span class="marker">{marker(i, player.currentIndex, player.state)}</span>
				<span class="num">{i + 1}.</span>
				<span class="title">{item.title}</span>
				<button
					class="icon row-del"
					title="삭제"
					on:click|stopPropagation={() => removeTrack(i)}
					on:dblclick|stopPropagation
				>🗑️</button>
			</li>
		{:else}
			<li class="placeholder">플레이리스트가 비어 있습니다. YouTube URL을 추가하세요.</li>
		{/each}
	</ul>

	<div class="transport">
		<button on:click={previous} title="이전 (←)">⏮ 이전</button>
		<button class="primary wide" on:click={togglePlay} disabled={isLoading} title="재생/일시정지 (Space)">
			{#if isPlaying}⏸ 일시정지{:else}▶ 재생{/if}
		</button>
		<button on:click={stop} title="정지">⏹ 정지</button>
		<button on:click={next} title="다음 (→)">⏭ 다음</button>
	</div>

	<div class="controls">
		<button class="loop" class:active={player.loopMode !== "none"} on:click={cycleLoop} title="반복 모드">
			{LOOP_LABELS[player.loopMode] ?? player.loopMode}
		</button>
		<label class="check" title="체크하면 영상을 별도 창으로 보여줍니다 (V)">
			<input type="checkbox" checked={player.showVideo} on:change={toggleVideo} on:keydown|stopPropagation />
			📺 화면 보기
		</label>
		<button on:click={() => removeTrack(selected)} disabled={selected < 0} title="선택 항목 삭제 (Delete)">
			🗑️ 삭제
		</button>
		<label class="volume">
			<span>볼륨: {volume}%</span>
			<input
				type="range"
				min="0"
				max="100"
				value={volume}
				on:input={(e) => setVolume(+e.currentTarget.value)}
				on:keydown|stopPropagation
			/>
		</label>
	</div>

	<div class="status">
		<span class="badge {player.state}">
			{#if isLoading}<span class="spinner"></span>{/if}
			{statusText}
		</span>
		<span class="now" title={current?.title ?? ""}>{current ? current.title : "—"}</span>
		{#if version}<span class="ver">v{version}</span>{/if}
	</div>

	{#if notice}
		<div class="notice" role="status">{notice}</div>
	{/if}

	{#if error}
		<div class="error" role="alert">
			<span>{error}</span>
			<button class="icon" title="닫기" on:click={() => (error = "")}>✕</button>
		</div>
	{/if}
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: 10px;
		height: 100vh;
		padding: 12px;
	}

	button {
		font: inherit;
		color: var(--text);
		background: var(--panel-2);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 7px 12px;
		cursor: pointer;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
	}

	button:hover:not(:disabled) {
		border-color: var(--accent);
	}

	.check {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
		user-select: none;
		white-space: nowrap;
	}

	.check input {
		width: 16px;
		height: 16px;
		margin: 0;
		accent-color: var(--accent, #4f8cff);
		cursor: pointer;
	}

	.notice {
		padding: 8px 12px;
		border-radius: 8px;
		background: rgba(79, 156, 249, 0.15);
		color: var(--accent, #4f9cf9);
		font-size: 13px;
	}

	button:disabled {
		opacity: 0.5;
		cursor: default;
	}

	button.primary {
		background: var(--accent-2);
		border-color: var(--accent);
	}

	button.primary:hover:not(:disabled) {
		background: var(--accent);
	}

	button.icon {
		background: transparent;
		border: none;
		padding: 2px 6px;
	}

	button.small {
		padding: 3px 10px;
	}

	.banner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		background: #1f3d2b;
		border: 1px solid var(--ok);
		border-radius: 6px;
	}

	.banner span {
		flex: 1;
	}

	.add-row {
		display: flex;
		gap: 8px;
	}

	.add-row input {
		flex: 1;
		min-width: 0;
		font: inherit;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 8px 10px;
		outline: none;
		user-select: text;
		-webkit-user-select: text;
	}

	.add-row input:focus {
		border-color: var(--accent);
	}

	.playlist {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		margin: 0;
		padding: 4px;
		list-style: none;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 6px;
	}

	.playlist li {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px;
		border-radius: 4px;
		cursor: default;
	}

	.playlist li:hover {
		background: rgba(255, 255, 255, 0.04);
	}

	.playlist li.selected {
		background: rgba(79, 156, 249, 0.22);
	}

	.playlist li.current .title {
		color: #8cc4ff;
		font-weight: 700;
	}

	.playlist li.placeholder {
		justify-content: center;
		color: var(--muted);
		padding: 24px 8px;
	}

	.marker {
		width: 1.2em;
		text-align: center;
		color: var(--accent);
	}

	.num {
		color: var(--muted);
		min-width: 2em;
		text-align: right;
	}

	.title {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row-del {
		opacity: 0;
	}

	.playlist li:hover .row-del,
	.playlist li.selected .row-del {
		opacity: 0.8;
	}

	.transport {
		display: grid;
		grid-template-columns: 1fr 1.4fr 1fr 1fr;
		gap: 8px;
	}

	.controls {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.loop.active {
		border-color: var(--accent);
		color: #8cc4ff;
	}

	.volume {
		flex: 1;
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 160px;
	}

	.volume span {
		min-width: 6.5em;
		color: var(--muted);
	}

	.volume input {
		flex: 1;
		accent-color: var(--accent);
	}

	.status {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		min-height: 34px;
	}

	.badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 12px;
		padding: 2px 8px;
		border-radius: 10px;
		background: var(--panel-2);
		color: var(--muted);
		flex-shrink: 0;
	}

	.badge.playing {
		background: rgba(63, 185, 80, 0.2);
		color: var(--ok);
	}

	.badge.loading {
		background: rgba(79, 156, 249, 0.2);
		color: var(--accent);
	}

	.badge.paused {
		background: rgba(210, 153, 34, 0.2);
		color: var(--warn);
	}

	.now {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.ver {
		color: var(--muted);
		font-size: 11px;
	}

	.error {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 8px 10px;
		background: rgba(229, 83, 75, 0.15);
		border: 1px solid var(--danger);
		border-radius: 6px;
		color: #ffb4ae;
		max-height: 30vh;
		overflow-y: auto;
	}

	.error span {
		flex: 1;
		white-space: pre-wrap;
		word-break: break-word;
		user-select: text;
		-webkit-user-select: text;
	}

	.spinner {
		width: 12px;
		height: 12px;
		border: 2px solid currentColor;
		border-right-color: transparent;
		border-radius: 50%;
		display: inline-block;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
