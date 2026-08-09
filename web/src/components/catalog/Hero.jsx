import { watchPath } from "../../lib/api";

export default function Hero({ item }) {
  if (!item) return null;

  return (
    <section className="hero">
      <div className="hero-media">
        {item.thumbnailUrl && <img className="hero-thumb" src={item.thumbnailUrl} alt="" />}
        <div className="hero-scrim" />
      </div>
      <div className="hero-content">
        <h1 className="hero-title">{item.name}</h1>
        <div className="hero-actions">
          <a className="btn btn-primary" href={watchPath(item)}>
            <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><path fill="currentColor" d="M8 5v14l11-7z" /></svg>
            Assistir
          </a>
        </div>
      </div>
    </section>
  );
}
