import { useRef } from "react";
import Card from "./Card";

export default function Row({ title, items }) {
  const trackRef = useRef(null);

  const scrollByCards = (dir) => {
    const track = trackRef.current;
    if (!track) return;
    const first = track.firstElementChild;
    const cardWidth = first ? first.getBoundingClientRect().width + 8 : 200;
    track.scrollBy({ left: dir * cardWidth * 4, behavior: "smooth" });
  };

  return (
    <section className="row">
      <h2 className="row-title">{title}</h2>
      <div className="row-track-wrap">
        <button className="row-nav row-nav-prev" type="button" aria-label="Anterior" onClick={() => scrollByCards(-1)}>
          ‹
        </button>
        <div className="row-track" ref={trackRef}>
          {items.map((item, i) => (
            <Card key={item.id} item={item} index={i} />
          ))}
        </div>
        <button className="row-nav row-nav-next" type="button" aria-label="Próximo" onClick={() => scrollByCards(1)}>
          ›
        </button>
      </div>
    </section>
  );
}
