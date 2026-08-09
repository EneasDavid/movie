import { Link } from "react-router-dom";
import { watchPath } from "../../lib/api";
import { readProgress, NEAR_END_RATIO } from "../../lib/progress";

export default function Card({ item, index }) {
  const progress = readProgress(item.id);
  let progressPct = null;
  if (progress && progress.duration > 0) {
    const ratio = Math.min(1, progress.time / progress.duration);
    if (ratio > 0.02 && ratio < NEAR_END_RATIO) progressPct = Math.round(ratio * 100);
  }

  return (
    <Link className="card" to={watchPath(item)} style={{ "--i": index % 12 }}>
      <div className="card-thumb-wrap">
        {item.thumbnailUrl && <img className="card-thumb" src={item.thumbnailUrl} alt="" loading="lazy" />}
        {progressPct !== null && (
          <div className="card-progress">
            <span className="card-progress-fill" style={{ width: `${progressPct}%` }} />
          </div>
        )}
        <div className="card-hover-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" width="28" height="28"><path fill="currentColor" d="M8 5v14l11-7z" /></svg>
        </div>
      </div>
      <div className="card-title">{item.name}</div>
    </Link>
  );
}
