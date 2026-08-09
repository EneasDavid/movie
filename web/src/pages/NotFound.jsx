import { useNavigate } from "react-router-dom";

// A long time ago, in a codebase far, far away... this route didn't exist.
// The shapes below are plain geometric silhouettes — a nod, not a redraw
// of any official design.
export default function NotFound() {
  const navigate = useNavigate();

  // navigate(-1) silently does nothing if this page has no history to go
  // back to (a bad URL typed/pasted directly, no prior in-app page) —
  // fall back to the catalog so the button always does something.
  const goBack = () => {
    if (window.history.length > 1) {
      navigate(-1);
    } else {
      navigate("/");
    }
  };

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
