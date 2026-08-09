import { useState } from "react";
import { Link } from "react-router-dom";
import { watchPath } from "../../lib/api";
import { readProgress, NEAR_END_RATIO } from "../../lib/progress";
import { formatTitle } from "../../lib/titleFormat";

export default function Card({ item, index }) {
  const [loaded, setLoaded] = useState(false);
  const progress = readProgress(item.id);
  let progressPct = null;
  if (progress && progress.duration > 0) {
    const ratio = Math.min(1, progress.time / progress.duration);
    if (ratio > 0.02 && ratio < NEAR_END_RATIO) progressPct = Math.round(ratio * 100);
  }

  return (
    <Link className="card" to={watchPath(item)} style={{ "--i": index % 12 }} title={formatTitle(item.name)}>
      <div className="card-thumb-wrap">
        {item.thumbnailUrl && (
          <img
            className={`card-thumb${loaded ? " loaded" : ""}`}
            src={item.thumbnailUrl}
            alt=""
            loading="lazy"
            onLoad={() => setLoaded(true)}
          />
        )}
        {progressPct !== null && (
          <div className="card-progress">
            <span className="card-progress-fill" style={{ width: `${progressPct}%` }} />
          </div>
        )}
        <div className="card-hover-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" width="28" height="28"><path fill="currentColor" d="M8 5v14l11-7z" /></svg>
        </div>
      </div>
      <div className="card-title">{formatTitle(item.name)}</div>
    </Link>
  );
}
