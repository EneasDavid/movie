import { useNavigate } from "react-router-dom";

// A long time ago, in a codebase far, far away... this route didn't exist.
// The shapes below are plain geometric silhouettes — a nod, not a redraw
// of any official design.
export default function NotFound() {
  const navigate = useNavigate();

  // Previously tried navigate(-1) when window.history.length > 1, falling
  // back to "/" otherwise — but history.length counts the browser tab's
  // whole session, not just in-app navigation, and mobile/webview browsers
  // often report it as >1 even with no real previous page here. That sent
  // "Voltar à base" back out of the app instead of to the catalog, which
  // looked like the button doing nothing. The button says "back to base",
  // so just always go to the base — no history heuristics needed.
  const goBack = () => navigate("/");

  return (
    <div className="notfound">
      <div className="notfound-nebula" aria-hidden="true" />
      <div className="notfound-stars" aria-hidden="true" />
      <div className="notfound-asteroids" aria-hidden="true" />

      <svg className="notfound-ship notfound-ship-station" viewBox="0 0 160 160" aria-hidden="true">
        <circle cx="80" cy="80" r="70" />
        <circle cx="80" cy="80" r="22" className="notfound-ship-detail" />
        <line x1="10" y1="80" x2="150" y2="80" className="notfound-ship-detail" />
      </svg>

      <svg className="notfound-ship notfound-ship-falcon" viewBox="0 0 200 140" aria-hidden="true">
        <polygon points="100,10 140,45 170,55 170,85 140,95 100,130 60,95 30,85 30,55 60,45" />
        <circle cx="100" cy="70" r="16" className="notfound-ship-detail" />
      </svg>

      <h1 className="notfound-title">
        Erro 404: Transmissão interrompida. O conteúdo que você procura se perdeu no hiperespaço ou foi removido da
        HoloNet por forças imperiais.
      </h1>
      <p className="notfound-subtitle">Use seu instinto da Força para retornar.</p>

      <button className="btn btn-primary" type="button" onClick={goBack}>
        Voltar à base
      </button>
    </div>
  );
}
