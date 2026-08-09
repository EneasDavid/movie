import { useCallback, useEffect, useMemo, useState } from "react";
import Hero from "../components/catalog/Hero";
import Row from "../components/catalog/Row";
import CatalogSkeleton from "../components/catalog/CatalogSkeleton";
import { fetchCatalog, fetchProgress, removeProgress, resendVerification } from "../lib/api";
import { MIN_RESUMABLE_SECONDS, NEAR_END_RATIO } from "../lib/progress";
import { useAuth } from "../context/AuthContext";
import Avatar from "../components/auth/Avatar";

function pickHero(categories) {
  for (const cat of categories) {
    if (cat.items && cat.items.length > 0) {
      return cat.items.find((it) => it.thumbnailUrl) || cat.items[0];
    }
  }
  return null;
}

export default function Catalog() {
  const { user, logout } = useAuth();
  const [catalog, setCatalog] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState("");
  const [progressEntries, setProgressEntries] = useState([]);
  const [resendState, setResendState] = useState("idle"); // idle | sending | sent | error

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
    fetchProgress()
      .then((entries) => {
        if (!cancelled) setProgressEntries(entries);
      })
      .catch(() => {
        /* continue-watching is a nice-to-have — a failure here shouldn't block the catalog */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const categories = useMemo(() => catalog?.categories || [], [catalog]);

  const itemsById = useMemo(() => {
    const map = new Map();
    for (const cat of categories) {
      for (const item of cat.items || []) map.set(item.id, item);
    }
    return map;
  }, [categories]);

  const progressByFileId = useMemo(() => {
    const map = new Map();
    for (const e of progressEntries) map.set(e.fileId, { time: e.time, duration: e.duration });
    return map;
  }, [progressEntries]);

  const continueWatchingItems = useMemo(() => {
    return progressEntries
      .filter((e) => {
        if (e.time < MIN_RESUMABLE_SECONDS) return false;
        if (e.duration > 0 && e.time / e.duration >= NEAR_END_RATIO) return false;
        return itemsById.has(e.fileId);
      })
      .map((e) => itemsById.get(e.fileId));
  }, [progressEntries, itemsById]);

  const handleRemoveFromContinueWatching = useCallback((fileId) => {
    // Optimistic: the row updates immediately, we don't make the user
    // wait on a round-trip to see the card disappear.
    setProgressEntries((prev) => prev.filter((e) => e.fileId !== fileId));
    removeProgress(fileId).catch(() => {
      // Best-effort — if it failed server-side, it'll just reappear next
      // time the list is refetched. Not worth a disruptive error UI for.
    });
  }, []);

  const handleResendVerification = useCallback(async () => {
    setResendState("sending");
    try {
      await resendVerification();
      setResendState("sent");
    } catch {
      setResendState("error");
    }
  }, []);

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
        <a className="brand" href="/">df-orfeu</a>
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
        <div className="topbar-user">
          <Avatar hasAvatar={user?.hasAvatar} size={28} />
          <span className="topbar-email">{user?.firstName || user?.email}</span>
          <button className="topbar-logout" type="button" onClick={logout}>
            Sair
          </button>
        </div>
      </header>

      {user && !user.emailVerified && (
        <div className="verify-banner" role="status">
          {resendState === "sent" ? (
            <span>Email de confirmação reenviado — confira sua caixa de entrada.</span>
          ) : (
            <>
              <span>Confirme seu email para garantir o acesso à sua conta.</span>
              <button type="button" onClick={handleResendVerification} disabled={resendState === "sending"}>
                {resendState === "sending" ? "Enviando…" : "Reenviar confirmação"}
              </button>
            </>
          )}
        </div>
      )}

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
          {visibleContinueWatching.length > 0 && (
            <Row
              title="Continuar assistindo"
              items={visibleContinueWatching}
              progressByFileId={progressByFileId}
              onRemoveItem={handleRemoveFromContinueWatching}
            />
          )}
          {visibleCategories.map((cat) => (
            <Row key={cat.id} title={cat.title} items={cat.items} progressByFileId={progressByFileId} />
          ))}
        </div>
      </main>
    </>
  );
}
