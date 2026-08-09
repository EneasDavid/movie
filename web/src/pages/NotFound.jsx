import { useNavigate } from "react-router-dom";

// A long time ago, in a codebase far, far away... this route didn't exist.
// The ships below are plain geometric silhouettes — a nod, not a redraw
// of any official design.
export default function NotFound() {
  const navigate = useNavigate();

  return (
    <div className="notfound">
      <div className="notfound-stars" aria-hidden="true" />

      <svg className="notfound-ship notfound-ship-falcon" viewBox="0 0 200 140" aria-hidden="true">
        <polygon points="100,10 140,45 170,55 170,85 140,95 100,130 60,95 30,85 30,55 60,45" />
        <circle cx="100" cy="70" r="16" />
      </svg>

      <svg className="notfound-ship notfound-ship-destroyer" viewBox="0 0 220 140" aria-hidden="true">
        <polygon points="110,10 210,120 150,120 110,95 70,120 10,120" />
      </svg>

      <h1 className="notfound-title">
        Erro 404: este planeta ainda não foi destruído nesta galáxia — a rota que você procura simplesmente não existe aqui.
      </h1>
      <p className="notfound-subtitle">Estes não são os endpoints que você procura.</p>

      <button className="btn btn-primary" type="button" onClick={() => navigate(-1)}>
        Voltar
      </button>
    </div>
  );
}
