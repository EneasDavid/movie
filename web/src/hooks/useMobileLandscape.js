import { useCallback, useEffect } from "react";

const MOBILE_QUERY = "(max-width: 900px)";

// Mobile video-watching comfort: force landscape while on the watch page.
//
// Two native layers, tried in order, because no single one is reliable across
// real devices:
//  1. `video.webkitEnterFullscreen()` — iOS Safari's native video
//     fullscreen. This is the ONE reliable fullscreen path on iOS: the OS
//     itself rotates the screen and owns the whole surface, so there's no
//     leftover status bar and no collision with system overlays (e.g. the
//     volume HUD) the way the CSS rotate fallback can have, since that
//     fallback doesn't change what the OS thinks the orientation is.
//     Trades our custom controls for Apple's native ones while active —
//     an acceptable swap for a real, glitch-free fullscreen.
//  2. `element.requestFullscreen()` — the standard API. Works on Android
//     Chrome and modern desktop browsers; iOS Safari only gained this for
//     arbitrary elements in 16.4, and even then behaves inconsistently
//     for a <div> wrapping <video>, so it's the second choice, not first,
//     on iOS.
export function useMobileLandscape(elementRef, videoRef) {
  useEffect(() => {
    if (!window.matchMedia(MOBILE_QUERY).matches) return undefined;

    return () => {
      try {
        screen.orientation?.unlock?.();
      } catch {
        /* not supported — nothing to undo */
      }
      if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
      }
    };
  }, []);

  const requestLandscape = useCallback(async () => {
    if (!window.matchMedia(MOBILE_QUERY).matches) return;

    const video = videoRef?.current;
    // iOS Safari: native video fullscreen. Real, OS-level, rotates and
    // hides system chrome correctly — try this first and skip the rest
    // if it works.
    if (video && typeof video.webkitEnterFullscreen === "function") {
      try {
        video.webkitEnterFullscreen();
        return;
      } catch {
        /* fall through to the generic Fullscreen API */
      }
    }

    const el = elementRef.current;
    try {
      if (el && !document.fullscreenElement) {
        await el.requestFullscreen();
      }
    } catch {
      /* denied/unsupported — keep the normal, unrotated player */
    }
    try {
      await screen.orientation?.lock?.("landscape");
    } catch {
      /* no Orientation Lock support (notably iOS) — expected */
    }
  }, [elementRef, videoRef]);

  return { requestLandscape };
}
