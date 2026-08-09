import { useState } from "react";
import { Link } from "react-router-dom";
import { watchPath } from "../../lib/api";
import { formatTitle } from "../../lib/titleFormat";

export default function Hero({ item }) {
  const [loaded, setLoaded] = useState(false);

  if (!item) return null;

  return (
    <section className="hero">
      <div className="hero-media">
        {item.thumbnailUrl && (
          <img
            className={`hero-thumb${loaded ? " loaded" : ""}`}
            src={item.thumbnailUrl}
            alt=""
            onLoad={() => setLoaded(true)}
          />
        )}
        <div className="hero-scrim" />
      </div>
      <div className="hero-content">
        <h1 className="hero-title">{formatTitle(item.name)}</h1>
        <div className="hero-actions">
          <Link className="btn btn-primary" to={watchPath(item)}>
            <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><path fill="currentColor" d="M8 5v14l11-7z" /></svg>
            Assistir
          </Link>
        </div>
      </div>
    </section>
  );
}
