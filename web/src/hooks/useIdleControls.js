import { useCallback, useEffect, useRef, useState } from "react";

// Tracks whether the player controls should be visible: true right after
// any pointer/keyboard activity, false after `idleMs` of no activity —
// but only while the video is actually playing (paused = controls stay
// put, nothing to hide from).
export function useIdleControls(isPlaying, idleMs = 2800) {
  const [idle, setIdle] = useState(false);
  const timerRef = useRef(null);

  const reset = useCallback(() => {
    setIdle(false);
    if (timerRef.current) clearTimeout(timerRef.current);
    if (isPlaying) {
      timerRef.current = setTimeout(() => setIdle(true), idleMs);
    }
  }, [isPlaying, idleMs]);

  useEffect(() => {
    reset();
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [reset]);

  useEffect(() => {
    if (!isPlaying && timerRef.current) {
      clearTimeout(timerRef.current);
      setIdle(false);
    }
  }, [isPlaying]);

  return { idle, resetIdle: reset };
}
