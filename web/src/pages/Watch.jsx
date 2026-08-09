import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import VideoPlayer from "../components/player/VideoPlayer";
import { formatTitle } from "../lib/titleFormat";

export default function Watch() {
  const [params] = useSearchParams();
  const fileId = params.get("id") || "";
  const title = formatTitle(params.get("title") || "");

  useEffect(() => {
    document.body.classList.add("player-body");
    document.title = title ? `${title} — MOVIE` : "MOVIE";
    return () => {
      document.body.classList.remove("player-body");
      document.title = "MOVIE";
    };
  }, [title]);

  if (!fileId) {
    return (
      <div className="error-overlay" style={{ position: "fixed" }} role="alert">
        <p>Vídeo não especificado.</p>
        <a className="btn btn-primary" href="/">Voltar ao catálogo</a>
      </div>
    );
  }

  return <VideoPlayer fileId={fileId} title={title} />;
}
