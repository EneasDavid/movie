import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import VideoPlayer from "../components/player/VideoPlayer";
import { formatTitle } from "../lib/titleFormat";

export default function Watch() {
  const [params] = useSearchParams();
  const fileId = params.get("id") || "";
  const title = formatTitle(params.get("title") || "");
  const version = params.get("v") || "";
	const mimeType = params.get("type") || "";
	const durationMs = Number(params.get("duration")) || 0;

  useEffect(() => {
    document.body.classList.add("player-body");
    document.title = title ? `${title} — DF Orfeu` : "DF Orfeu";
    return () => {
      document.body.classList.remove("player-body");
      document.title = "DF Orfeu";
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

  return <VideoPlayer fileId={fileId} title={title} version={version} mimeType={mimeType} initialDuration={durationMs / 1000} />;
}
