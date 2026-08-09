import { useCallback, useRef } from "react";

function formatTime(seconds) {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

export default function Controls({
  idle,
  title,
  isPlaying,
  currentTime,
  duration,
  buffered,
  volume,
  muted,
  isFullscreen,
  onTogglePlay,
  onSeekBy,
  onSeekTo,
  onVolumeChange,
  onToggleMute,
  onToggleFullscreen,
}) {
  const barRef = useRef(null);

  const playedPct = duration > 0 ? (currentTime / duration) * 100 : 0;
  const bufferedPct = duration > 0 ? (buffered / duration) * 100 : 0;

  const seekFromEvent = useCallback(
    (clientX) => {
      const bar = barRef.current;
      if (!bar) return;
      const rect = bar.getBoundingClientRect();
      const fraction = (clientX - rect.left) / rect.width;
      onSeekTo(fraction);
    },
    [onSeekTo]
  );

  const handleBarClick = useCallback((e) => seekFromEvent(e.clientX), [seekFromEvent]);

  const handleBarDrag = useCallback(
    (e) => {
      e.preventDefault();
      seekFromEvent(e.clientX);
      const onMove = (moveEvent) => seekFromEvent(moveEvent.clientX);
      const onUp = () => {
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onUp);
      };
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onUp);
    },
    [seekFromEvent]
  );

  const handleBarKeyDown = useCallback(
    (e) => {
      if (e.key === "ArrowRight") onSeekBy(5);
      if (e.key === "ArrowLeft") onSeekBy(-5);
    },
    [onSeekBy]
  );

  return (
    <div className={`controls${idle ? " idle" : ""}`}>
      <div className="controls-top">
        <a className="icon-btn back-btn" href="/" aria-label="Voltar ao catálogo">
          <svg viewBox="0 0 24 24" width="26" height="26">
            <path fill="currentColor" d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20z" />
          </svg>
        </a>
        <h1 className="video-title">{title}</h1>
      </div>

      <div className="center-controls">
        <button className="icon-btn center-btn" type="button" onClick={() => onSeekBy(-15)} aria-label="Voltar 15 segundos">
          <svg viewBox="0 0 24 24" width="30" height="30">
            <path fill="currentColor" d="M13 3a9 9 0 0 0-9 9H1l4 4 4-4H6a7 7 0 1 1 2.05 4.95l-1.42 1.42A9 9 0 1 0 13 3z" />
          </svg>
          <span className="icon-badge">15</span>
        </button>

        <button className="icon-btn center-btn center-btn-play" type="button" onClick={onTogglePlay} aria-label={isPlaying ? "Pausar" : "Reproduzir"}>
          {isPlaying ? (
            <svg viewBox="0 0 24 24" width="40" height="40"><path fill="currentColor" d="M6 5h4v14H6zM14 5h4v14h-4z" /></svg>
          ) : (
            <svg viewBox="0 0 24 24" width="40" height="40"><path fill="currentColor" d="M8 5v14l11-7z" /></svg>
          )}
        </button>

        <button className="icon-btn center-btn" type="button" onClick={() => onSeekBy(15)} aria-label="Avançar 15 segundos">
          <svg viewBox="0 0 24 24" width="30" height="30" style={{ transform: "scaleX(-1)" }}>
            <path fill="currentColor" d="M13 3a9 9 0 0 0-9 9H1l4 4 4-4H6a7 7 0 1 1 2.05 4.95l-1.42 1.42A9 9 0 1 0 13 3z" />
          </svg>
          <span className="icon-badge">15</span>
        </button>
      </div>

      <div className="controls-bottom">
        <div className="progress-wrap">
          <div
            ref={barRef}
            className="progress-bar"
            role="slider"
            tabIndex={0}
            aria-label="Progresso do vídeo"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(playedPct)}
            onClick={handleBarClick}
            onPointerDown={handleBarDrag}
            onKeyDown={handleBarKeyDown}
          >
            <div className="progress-buffered" style={{ width: `${bufferedPct}%` }} />
            <div className="progress-played" style={{ width: `${playedPct}%` }} />
            <div className="progress-handle" style={{ left: `${playedPct}%` }} />
          </div>
        </div>

        <div className="controls-row">
          <div className="controls-left">
            <div className="volume-group">
              <button className="icon-btn" type="button" onClick={onToggleMute} aria-label={muted ? "Ativar som" : "Mudo"}>
                {muted || volume === 0 ? (
                  <svg viewBox="0 0 24 24" width="24" height="24"><path fill="currentColor" d="M16.5 12A4.5 4.5 0 0 0 14 7.97v2.21l2.45 2.45c.03-.2.05-.42.05-.63zM19 12c0 .94-.2 1.82-.54 2.64l1.51 1.51A8.9 8.9 0 0 0 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3 3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06a8.99 8.99 0 0 0 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4l-1.88 1.88L12 7.76V4z" /></svg>
                ) : (
                  <svg viewBox="0 0 24 24" width="24" height="24"><path fill="currentColor" d="M3 10v4h4l5 5V5L7 10H3zm13.5 2A4.5 4.5 0 0 0 14 7.97v8.05A4.5 4.5 0 0 0 16.5 12z" /></svg>
                )}
              </button>
              <input
                className="volume-slider"
                type="range"
                min="0"
                max="1"
                step="0.05"
                value={muted ? 0 : volume}
                onChange={(e) => onVolumeChange(Number(e.target.value))}
                aria-label="Volume"
              />
            </div>

            <div className="time-display">
              <span>{formatTime(currentTime)}</span>
              <span className="time-sep">/</span>
              <span>{formatTime(duration)}</span>
            </div>
          </div>

          <div className="controls-right">
            <button className="icon-btn" type="button" onClick={onToggleFullscreen} aria-label={isFullscreen ? "Sair da tela cheia" : "Tela cheia"}>
              <svg viewBox="0 0 24 24" width="24" height="24">
                <path fill="currentColor" d="M7 14H5v5h5v-2H7zm-2-4h2V7h3V5H5zm12 7h-3v2h5v-5h-2zM14 5v2h3v3h2V5z" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
