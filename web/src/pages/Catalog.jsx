import { useEffect, useMemo, useState } from "react";
import Hero from "../components/catalog/Hero";
import Row from "../components/catalog/Row";
import CatalogSkeleton from "../components/catalog/CatalogSkeleton";
import { fetchCatalog } from "../lib/api";
import { allProgressEntries, MIN_RESUMABLE_SECONDS, NEAR_END_RATIO } from "../lib/progress";

function pickHero(categories) {
  for (const cat of categories) {
    if (cat.items && cat.items.length > 0) {
      return cat.items.find((it) => it.thumbnailUrl) || cat.items[0];
    }
  }
  return null;
}

export default function Catalog() {
  const [catalog, setCatalog] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchCatalog()
      .then((data) => {
        if (!cancelled) setCatalog(data);
      })
      .catch((err) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const categories = catalog?.categories || [];

  const itemsById = useMemo(() => {
    const map = new Map();
    for (const cat of categories) {
      for (const item of cat.items || []) map.set(item.id, item);
    }
    return map;
  }, [categories]);

  const continueWatchingItems = useMemo(() => {
    const entries = allProgressEntries().filter((e) => {
      if (e.time < MIN_RESUMABLE_SECONDS) return false;
      if (e.duration > 0 && e.time / e.duration >= NEAR_END_RATIO) return false;
      return itemsById.has(e.fileId);
    });
    return entries.map((e) => itemsById.get(e.fileId));
  }, [itemsById]);

  const hero = useMemo(() => pickHero(categories), [categories]);

  const q = query.trim().toLowerCase();
  const filterItems = (items) => (q ? items.filter((it) => (it.name || "").toLowerCase().includes(q)) : items);

  const visibleCategories = categories
    .map((cat) => ({ ...cat, items: filterItems(cat.items || []) }))
    .filter((cat) => cat.items.length > 0);

  const visibleContinueWatching = filterItems(continueWatchingItems);

  const nothingToShow = !loading && !error && visibleCategories.length === 0 && visibleContinueWatching.length === 0;

  return (
    <>
      <header className="topbar">
        <a className="brand" href="/">MOVIE</a>
        <div className="search">
          <input
            className="search-input"
            type="search"
            placeholder="Buscar títulos..."
            aria-label="Buscar títulos"
            autoComplete="off"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
      </header>

      <main>
        {!q && <Hero item={hero} />}

        {loading && (
          <>
            <div className="sr-only" role="status" aria-live="polite">Carregando catálogo…</div>
            <CatalogSkeleton />
          </>
        )}
        {error && (
          <div className="status error" role="alert">
            Não foi possível carregar o catálogo: {error}
          </div>
        )}
        {nothingToShow && (
          <div className="status error">
            {q
              ? "Nenhum título encontrado para essa busca."
              : "Nenhum vídeo encontrado. Verifique se a pasta do Google Drive contém arquivos de vídeo e está compartilhada como 'Qualquer pessoa com o link'."}
          </div>
        )}

        <div className="rows">
          {visibleContinueWatching.length > 0 && <Row title="Continuar assistindo" items={visibleContinueWatching} />}
          {visibleCategories.map((cat) => (
            <Row key={cat.id} title={cat.title} items={cat.items} />
          ))}
        </div>
      </main>
    </>
  );
}
