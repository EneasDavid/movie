import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { streamURL } from "../../lib/api";
import { readProgress, writeProgress, MIN_RESUMABLE_SECONDS, NEAR_END_RATIO } from "../../lib/progress";
import { useIdleControls } from "../../hooks/useIdleControls";
import Controls from "./Controls";
import CenterFeedback from "./CenterFeedback";

const SEEK_SECONDS = 15;
const PROGRESS_SAVE_INTERVAL_MS = 5000;

export default function VideoPlayer({ fileId, title }) {
  const videoRef = useRef(null);
  const playerRef = useRef(null);
  const lastSaveRef = useRef(0);
  const resumedRef = useRef(false);

  const [isPlaying, setIsPlaying] = useState(false);
  const [isBuffering, setIsBuffering] = useState(true);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [buffered, setBuffered] = useState(0);
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [error, setError] = useState(null);
  const [feedback, setFeedback] = useState(null); // { type: 'play'|'pause'|'back15'|'fwd15' }

  const { idle, resetIdle } = useIdleControls(isPlaying);
  const src = useMemo(() => streamURL(fileId), [fileId]);

  const flashFeedback = useCallback((type) => {
    setFeedback({ type, key: Date.now() });
  }, []);

  // --- Resume from saved progress once metadata is known ---
  const handleLoadedMetadata = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    setDuration(video.duration || 0);
    if (!resumedRef.current) {
      resumedRef.current = true;
      const saved = readProgress(fileId);
      if (saved && saved.time >= MIN_RESUMABLE_SECONDS && saved.time / (saved.duration || video.duration || 1) < NEAR_END_RATIO) {
        video.currentTime = saved.time;
      }
    }
  }, [fileId]);

  const saveProgress = useCallback(
    (force) => {
      const video = videoRef.current;
      if (!video || !video.duration) return;
      const now = Date.now();
      if (!force && now - lastSaveRef.current < PROGRESS_SAVE_INTERVAL_MS) return;
      lastSaveRef.current = now;
      writeProgress(fileId, video.currentTime, video.duration, title);
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

  // --- Core actions ---
  const togglePlay = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    if (video.paused) {
      video.play().catch(() => setError("Não foi possível iniciar a reprodução."));
    } else {
      video.pause();
    }
    resetIdle();
  }, [resetIdle]);

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
      setError(
        code === 4
          ? "Vídeo indisponível: verifique se o arquivo ainda está compartilhado no Google Drive."
          : "Ocorreu um erro ao carregar o vídeo."
      );
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
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={videoRef}
        className="video"
        src={src}
        playsInline
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
