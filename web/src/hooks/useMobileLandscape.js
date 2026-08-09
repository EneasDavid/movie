import { useCallback, useEffect, useState } from "react";

const MOBILE_QUERY = "(max-width: 900px)";

// Mobile video-watching comfort: force landscape while on the watch page.
//
// Two layers, because neither alone is reliable across real devices:
//  1. A CSS rotate transform (`forceRotateCss`) — works everywhere,
//     needs no permission, applied whenever the viewport is mobile-width
//     AND still physically portrait. No-ops itself the moment the real
//     orientation becomes landscape (device physically rotated, or layer
//     2 below succeeded), via a live matchMedia listener.
//  2. `requestLandscape()` — Fullscreen API + Screen Orientation Lock.
//     Only works from within a genuine user gesture (a tap), and Screen
//     Orientation Lock has no Safari/iOS support at all — so this is
//     best-effort, called again on the first tap where CSS-rotate alone
//     already covers the same need in the meantime.
export function useMobileLandscape(elementRef) {
  const [forceRotateCss, setForceRotateCss] = useState(false);

  useEffect(() => {
    if (!window.matchMedia(MOBILE_QUERY).matches) return undefined;

    const portraitQuery = window.matchMedia("(orientation: portrait)");
    const update = () => setForceRotateCss(portraitQuery.matches);
    update();
    portraitQuery.addEventListener("change", update);

    return () => {
      portraitQuery.removeEventListener("change", update);
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
    const el = elementRef.current;
    try {
      if (el && !document.fullscreenElement) {
        await el.requestFullscreen();
      }
    } catch {
      /* denied/unsupported — CSS rotate fallback still covers it */
    }
    try {
      await screen.orientation?.lock?.("landscape");
    } catch {
      /* iOS Safari has no Orientation Lock API at all — expected */
    }
  }, [elementRef]);

  return { forceRotateCss, requestLandscape };
}
