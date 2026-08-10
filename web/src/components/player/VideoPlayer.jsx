import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { streamURL, fetchProgress, saveProgress as saveProgressRemote } from "../../lib/api";
import { MIN_RESUMABLE_SECONDS, NEAR_END_RATIO } from "../../lib/progress";
import { useIdleControls } from "../../hooks/useIdleControls";
import { useMobileLandscape } from "../../hooks/useMobileLandscape";
import Controls from "./Controls";
import CenterFeedback from "./CenterFeedback";

const SEEK_SECONDS = 15;
const PROGRESS_SAVE_INTERVAL_MS = 5000;

export default function VideoPlayer({ fileId, title, version }) {
  const videoRef = useRef(null);
  const playerRef = useRef(null);
  const lastSaveRef = useRef(0);
  const resumedRef = useRef(false);
  const initialProgressRef = useRef(null);

  const [isPlaying, setIsPlaying] = useState(false);
  const [isBuffering, setIsBuffering] = useState(true);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [buffered, setBuffered] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [error, setError] = useState(null);
  const [retryNonce, setRetryNonce] = useState(0);
  const [feedback, setFeedback] = useState(null); // { type: 'play'|'pause'|'back15'|'fwd15' }

  const { idle, resetIdle } = useIdleControls(isPlaying);
  const { requestLandscape } = useMobileLandscape(playerRef, videoRef);
  const src = useMemo(() => streamURL(fileId, retryNonce ? `${version}-${retryNonce}` : version), [fileId, version, retryNonce]);

  const flashFeedback = useCallback((type) => {
    setFeedback({ type, key: Date.now() });
  }, []);

  // Seeks to the saved position — a no-op until both pieces it needs are
  // ready: the fetched progress entry (async, from the server) and the
  // video's own duration (async, from the browser). Whichever of the two
  // effects below finishes second is what actually triggers the seek;
  // resumedRef guards against doing it twice.
  const maybeResume = useCallback(() => {
    const video = videoRef.current;
    const saved = initialProgressRef.current;
    if (!video || !saved || resumedRef.current || !video.duration) return;
    resumedRef.current = true;
    if (saved.time >= MIN_RESUMABLE_SECONDS && saved.time / (saved.duration || video.duration || 1) < NEAR_END_RATIO) {
      video.currentTime = saved.time;
    }
  }, []);

  // Fetch saved progress from the server as soon as the player mounts.
  useEffect(() => {
    let cancelled = false;
    fetchProgress()
      .then((entries) => {
        if (cancelled) return;
        initialProgressRef.current = entries.find((e) => e.fileId === fileId) || null;
        maybeResume();
      })
      .catch(() => {
        /* no saved progress available — just start from the beginning */
      });
    return () => {
      cancelled = true;
    };
  }, [fileId, maybeResume]);

  const handleLoadedMetadata = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    setDuration(video.duration || 0);
    maybeResume();
  }, [maybeResume]);

  const saveProgress = useCallback(
    (force) => {
      const video = videoRef.current;
      if (!video || !video.duration) return;
      const now = Date.now();
      if (!force && now - lastSaveRef.current < PROGRESS_SAVE_INTERVAL_MS) return;
      lastSaveRef.current = now;
      saveProgressRemote(fileId, video.currentTime, video.duration, title).catch(() => {
        /* best-effort — a dropped save just means resume falls back a
           little further next time, not worth surfacing to the viewer */
      });
    },
    [fileId, title]
  );

  const handleTimeUpdate = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    setCurrentTime(video.currentTime);
    if (video.buffered.length > 0) {
      setBuffered(video.buffered.end(video.buffered.length - 1));
    }
    saveProgress(false);
  }, [saveProgress]);

  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState === "hidden") saveProgress(true);
    };
    window.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("pagehide", onVisibility);
    return () => {
      saveProgress(true);
      window.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("pagehide", onVisibility);
    };
  }, [saveProgress]);

  // Best-effort: if the card tap that navigated here still counts as an
  // active user gesture by the time this runs, fullscreen+landscape lock
  // succeeds immediately with no second tap needed. If not, this silently
  // no-ops and the guaranteed retry in togglePlay (a real, fresh tap)
  // picks it up. We deliberately do not rotate with CSS: that leaves the
  // iOS status bar and volume HUD in portrait while the page is sideways.
  useEffect(() => {
    requestLandscape();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // --- Core actions ---
  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      video.play().catch(() => setError("Não foi possível iniciar a reprodução."));
      // Tied to this tap so it counts as a real user gesture — required
      // by both the Fullscreen API and Screen Orientation Lock. Safe to
      // call every play tap: entering fullscreen/locking twice is a no-op.
      requestLandscape();
    } else {
      video.pause();
    }
    resetIdle();
  }, [resetIdle, requestLandscape]);

  const seekBy = useCallback(
    (deltaSeconds) => {
      const video = videoRef.current;
      if (!video || !video.duration) return;
      video.currentTime = Math.min(Math.max(0, video.currentTime + deltaSeconds), video.duration);
      flashFeedback(deltaSeconds < 0 ? "back15" : "fwd15");
      resetIdle();
    },
    [flashFeedback, resetIdle]
  );

  const seekTo = useCallback(
    (fraction) => {
      const video = videoRef.current;
      if (!video || !video.duration) return;
      video.currentTime = Math.min(Math.max(0, fraction), 1) * video.duration;
      resetIdle();
    },
    [resetIdle]
  );

  const setVolumeSafe = useCallback((v) => {
    const video = videoRef.current;
    if (!video) return;
    video.volume = Math.min(Math.max(0, v), 1);
    setVolume(video.volume);
    if (video.volume > 0 && video.muted) video.muted = false;
  }, []);

  const toggleMute = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    video.muted = !video.muted;
    setMuted(video.muted);
    resetIdle();
  }, [resetIdle]);

  const toggleFullscreen = useCallback(() => {
    const el = playerRef.current;
    if (!el) return;
    if (!document.fullscreenElement) {
      el.requestFullscreen?.().catch(() => {});
    } else {
      document.exitFullscreen?.().catch(() => {});
    }
    resetIdle();
  }, [resetIdle]);

  const retryPlayback = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    const nonce = Date.now();
    const retrySrc = streamURL(fileId, `${version}-${nonce}`);
    setError(null);
    setIsBuffering(true);
    setRetryNonce(nonce);
    // Assign synchronously inside the button gesture so iOS permits both
    // the new media request and native fullscreen playback.
    video.src = retrySrc;
    video.load();
    video.play().catch(() => setError("Não foi possível iniciar o vídeo. Verifique a conexão e tente novamente."));
    requestLandscape();
  }, [fileId, requestLandscape, version]);

  // --- Video element event wiring ---
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const onPlay = () => {
      setIsPlaying(true);
      flashFeedback("play");
    };
    const onPause = () => {
      setIsPlaying(false);
      flashFeedback("pause");
      saveProgress(true);
    };
    const onWaiting = () => setIsBuffering(true);
    const onPlaying = () => setIsBuffering(false);
    const onCanPlay = () => setIsBuffering(false);
    const onVolumeChange = () => {
      setVolume(video.volume);
      setMuted(video.muted);
    };
    const onError = () => {
      const code = video.error?.code;
      const messages = {
        2: "A conexão com o vídeo foi interrompida. Tente novamente.",
        3: "O navegador não conseguiu decodificar os dados recebidos. Tente recarregar o vídeo.",
        4: "O navegador não conseguiu abrir o stream de vídeo. Tente novamente para buscar a versão atualizada.",
      };
      setError(messages[code] || "Ocorreu um erro ao carregar o vídeo.");
      setIsBuffering(false);
    };
    const onEnded = () => saveProgress(true);

    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("waiting", onWaiting);
    video.addEventListener("playing", onPlaying);
    video.addEventListener("canplay", onCanPlay);
    video.addEventListener("volumechange", onVolumeChange);
    video.addEventListener("error", onError);
    video.addEventListener("ended", onEnded);
    video.addEventListener("loadedmetadata", handleLoadedMetadata);
    video.addEventListener("timeupdate", handleTimeUpdate);

    return () => {
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("waiting", onWaiting);
      video.removeEventListener("playing", onPlaying);
      video.removeEventListener("canplay", onCanPlay);
      video.removeEventListener("volumechange", onVolumeChange);
      video.removeEventListener("error", onError);
      video.removeEventListener("ended", onEnded);
      video.removeEventListener("loadedmetadata", handleLoadedMetadata);
      video.removeEventListener("timeupdate", handleTimeUpdate);
    };
  }, [flashFeedback, handleLoadedMetadata, handleTimeUpdate, saveProgress]);

  useEffect(() => {
    const onFsChange = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", onFsChange);
    return () => document.removeEventListener("fullscreenchange", onFsChange);
  }, []);

  // --- Keyboard shortcuts ---
  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.target instanceof HTMLInputElement) return;
      switch (e.code) {
        case "Space":
        case "KeyK":
          e.preventDefault();
          togglePlay();
          break;
        case "ArrowLeft":
        case "KeyJ":
          e.preventDefault();
          seekBy(-SEEK_SECONDS);
          break;
        case "ArrowRight":
        case "KeyL":
          e.preventDefault();
          seekBy(SEEK_SECONDS);
          break;
        case "ArrowUp":
          e.preventDefault();
          setVolumeSafe(volume + 0.1);
          break;
        case "ArrowDown":
          e.preventDefault();
          setVolumeSafe(volume - 0.1);
          break;
        case "KeyM":
          toggleMute();
          break;
        case "KeyF":
          toggleFullscreen();
          break;
        default:
          break;
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [togglePlay, seekBy, setVolumeSafe, toggleMute, toggleFullscreen, volume]);

  return (
    <div
      ref={playerRef}
      className="player"
      onMouseMove={resetIdle}
      onTouchStart={resetIdle}
    >
      <video
        ref={videoRef}
        className="video"
        src={src}
        preload="metadata"
        autoPlay
        onClick={togglePlay}
      />

      {isBuffering && !error && (
        <div className="center-spinner" aria-hidden="true">
          <div className="spinner" />
        </div>
      )}

      <CenterFeedback feedback={feedback} />

      {error && (
        <div className="error-overlay" role="alert">
          <p>{error}</p>
          <button className="btn btn-primary" type="button" onClick={retryPlayback}>
            Tentar novamente
          </button>
          <a className="btn btn-primary" href="/">
            Voltar ao catálogo
          </a>
        </div>
      )}

      {!error && (
        <Controls
          idle={idle}
          title={title}
          isPlaying={isPlaying}
          currentTime={currentTime}
          duration={duration}
          buffered={buffered}
          volume={volume}
          muted={muted}
          isFullscreen={isFullscreen}
          onTogglePlay={togglePlay}
          onSeekBy={seekBy}
          onSeekTo={seekTo}
          onVolumeChange={setVolumeSafe}
          onToggleMute={toggleMute}
          onToggleFullscreen={toggleFullscreen}
        />
      )}
    </div>
  );
}
