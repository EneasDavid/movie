// The brief icon that flashes center-screen on play/pause/seek — mirrors
// the classic streaming-player "tap feedback". Driven by a `key` change
// so React re-triggers the CSS transition every time, even for the same
// type fired twice in a row (e.g. two quick "back15" presses).
export default function CenterFeedback({ feedback }) {
  if (!feedback) return null;

  const icons = {
    play: <path fill="currentColor" d="M8 5v14l11-7z" />,
    pause: <path fill="currentColor" d="M6 5h4v14H6zM14 5h4v14h-4z" />,
    back15: <path fill="currentColor" d="M13 3a9 9 0 0 0-9 9H1l4 4 4-4H6a7 7 0 1 1 2.05 4.95l-1.42 1.42A9 9 0 1 0 13 3z" />,
    fwd15: <path fill="currentColor" d="M13 3a9 9 0 0 0-9 9H1l4 4 4-4H6a7 7 0 1 1 2.05 4.95l-1.42 1.42A9 9 0 1 0 13 3z" />,
  };

  return (
    <div className="center-feedback" key={feedback.key} aria-hidden="true">
      <svg viewBox="0 0 24 24" style={feedback.type === "fwd15" ? { transform: "scaleX(-1)" } : undefined}>
        {icons[feedback.type]}
      </svg>
    </div>
  );
}
