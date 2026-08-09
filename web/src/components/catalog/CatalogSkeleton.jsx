// Placeholder rows shown while /api/catalog is loading — replaces a blank
// screen (or a lone "Carregando…" line) with a shape that already looks
// like the real layout, which reads as faster even when it isn't.
export default function CatalogSkeleton() {
  return (
    <div className="rows" aria-hidden="true">
      {[0, 1, 2].map((row) => (
        <section className="row" key={row}>
          <div className="skeleton skeleton-title" />
          <div className="row-track-wrap">
            <div className="row-track">
              {[0, 1, 2, 3, 4, 5].map((i) => (
                <div className="card skeleton-card" key={i} style={{ "--i": i }}>
                  <div className="skeleton card-thumb-wrap" />
                </div>
              ))}
            </div>
          </div>
        </section>
      ))}
    </div>
  );
}
