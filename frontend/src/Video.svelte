<script>
	import { onDestroy, onMount } from "svelte";
	import { EventsOff, EventsOn } from "../wailsjs/runtime/runtime";

	// 영상 창: <video>(무음) + <audio>(기준 시계) 동기 재생
	//  - 영상에 소리가 포함된 단일 스트림이면 <video> 만 사용
	const report = (ev) => window.go.main.VideoApp.Report(ev);

	// 오디오와 영상 차이가 이보다 크면 영상 위치 보정 (초)
	const MAX_DRIFT = 0.3;

	let video;
	let audio;
	let seq = 0; // 0 = 정지
	let separate = false; // 영상/오디오 분리 스트림
	let wantPlaying = false;
	let bufferPaused = false; // 영상 버퍼링으로 오디오를 잠시 멈춤
	let lastProgress = 0;
	let message = "";

	const master = () => (separate ? audio : video);

	function playAll() {
		wantPlaying = true;
		const s = seq;
		const fail = (e) => {
			if (e?.name !== "AbortError" && s === seq) {
				report({ event: "failed", seq: s, message: String(e?.message ?? e) });
			}
		};
		video.play().catch(fail);
		if (separate) audio.play().catch(fail);
	}

	function pauseAll() {
		wantPlaying = false;
		video.pause();
		if (separate) audio.pause();
	}

	function onCommand(cmd) {
		if (!cmd) return;

		switch (cmd.action) {
			case "play":
				seq = cmd.seq;
				message = "";
				separate = Boolean(cmd.url);
				video.muted = separate;
				video.src = cmd.videoUrl;
				if (separate) {
					audio.src = cmd.url;
				} else {
					audio.removeAttribute("src");
					audio.load();
				}
				setVolume(cmd.volume);
				if (cmd.startAt > 0) {
					video.currentTime = cmd.startAt;
					if (separate) audio.currentTime = cmd.startAt;
				}
				playAll();
				break;
			case "pause":
				pauseAll();
				break;
			case "resume":
				playAll();
				break;
			case "stop":
				seq = 0;
				pauseAll();
				for (const el of [video, audio]) {
					el.removeAttribute("src");
					el.load();
				}
				break;
			case "volume":
				setVolume(cmd.volume);
				break;
		}
	}

	function setVolume(v) {
		(separate ? audio : video).volume = v / 100;
	}

	// ---- 이벤트 → 메인 ----

	const MEDIA_ERRORS = {
		1: "재생이 중단되었습니다",
		2: "네트워크 오류",
		3: "디코딩 실패",
		4: "지원하지 않는 형식이거나 스트림을 열 수 없습니다",
	};

	function onError(e) {
		if (!seq) return;
		const code = e.currentTarget.error?.code;
		report({ event: "failed", seq, message: MEDIA_ERRORS[code] ?? `미디어 오류 (${code})` });
	}

	function onEnded(e) {
		if (seq && e.currentTarget === master()) report({ event: "ended", seq });
	}

	function onTimeUpdate() {
		if (!seq) return;
		const m = master();

		// 영상 위치 보정
		if (separate && Math.abs(video.currentTime - audio.currentTime) > MAX_DRIFT) {
			video.currentTime = audio.currentTime;
		}

		if (Math.abs(m.currentTime - lastProgress) >= 1) {
			lastProgress = m.currentTime;
			report({ event: "progress", seq, time: m.currentTime });
		}
	}

	// 영상 버퍼링 중에는 오디오도 대기
	function onVideoWaiting() {
		if (separate && wantPlaying && !audio.paused) {
			bufferPaused = true;
			audio.pause();
		}
	}

	function onVideoPlaying() {
		if (separate && wantPlaying && audio.paused) {
			bufferPaused = false;
			audio.play().catch(() => {});
		}
	}

	// OS 가 일시정지한 경우 등 상태 동기화 (버퍼링 대기는 제외)
	function onMasterPause(e) {
		if (!seq || e.currentTarget !== master() || master().ended) return;
		if (bufferPaused) return;
		report({ event: "paused", seq, paused: true });
	}

	function onMasterPlaying(e) {
		if (seq && e.currentTarget === master()) report({ event: "paused", seq, paused: false });
	}

	// 창 안에서 클릭 / Space 로 일시정지
	function toggle() {
		if (seq) report({ event: "command", seq, action: "playpause" });
	}

	function onKey(e) {
		const actions = { " ": "playpause", ArrowRight: "next", ArrowLeft: "prev" };
		const action = actions[e.key];
		if (action && seq) {
			e.preventDefault();
			report({ event: "command", seq, action });
		}
	}

	// ---- MediaSession: 영상 창이 재생 중일 때 미디어 키는 이 창으로 들어옴 ----

	function setupMediaSession() {
		if (!("mediaSession" in navigator)) return;
		const send = (action) => () => seq && report({ event: "command", seq, action });
		const handlers = {
			play: send("playpause"),
			pause: send("playpause"),
			stop: send("stop"),
			nexttrack: send("next"),
			previoustrack: send("prev"),
		};
		for (const [action, fn] of Object.entries(handlers)) {
			try {
				navigator.mediaSession.setActionHandler(action, fn);
			} catch {
				// 지원하지 않는 액션
			}
		}
	}

	onMount(() => {
		EventsOn("video:command", onCommand);
		setupMediaSession();
		message = "영상 불러오는 중…";
		window.go.main.VideoApp.Ready();
	});

	onDestroy(() => EventsOff("video:command"));
</script>

<svelte:window on:keydown={onKey} />

<!-- svelte-ignore a11y-media-has-caption -->
<!-- svelte-ignore a11y-click-events-have-key-events -->
<div class="stage" on:click={toggle}>
	<video
		bind:this={video}
		playsinline
		preload="auto"
		on:ended={onEnded}
		on:error={onError}
		on:timeupdate={onTimeUpdate}
		on:waiting={onVideoWaiting}
		on:playing={(e) => {
			message = "";
			onVideoPlaying();
			onMasterPlaying(e);
		}}
		on:pause={onMasterPause}
		on:loadedmetadata={() => window.go.main.VideoApp.Fit(video.videoWidth, video.videoHeight, window.innerWidth, window.innerHeight)}
	/>
	{#if message}<p class="msg">{message}</p>{/if}
</div>

<audio
	bind:this={audio}
	preload="auto"
	on:ended={onEnded}
	on:error={onError}
	on:timeupdate={onTimeUpdate}
	on:playing={onMasterPlaying}
	on:pause={onMasterPause}
/>

<style>
	.stage {
		position: fixed;
		inset: 0;
		background: #000;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
	}

	video {
		width: 100%;
		height: 100%;
		object-fit: contain;
		background: #000;
	}

	.msg {
		position: absolute;
		color: #9aa4b2;
		font-size: 14px;
	}
</style>
