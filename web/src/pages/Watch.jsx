import { useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import VideoPlayer from "../components/player/VideoPlayer";

export default function Watch() {
  const [params] = useSearchParams();
  const fileId = params.get("id") || "";
  const title = params.get("title") || "";

  useEffect(() => {
    document.body.classList.add("player-body");
    return () => document.body.classList.remove("player-body");
  }, []);

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
