import { Route, Routes } from "react-router-dom";
import Catalog from "./pages/Catalog";
import Watch from "./pages/Watch";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Catalog />} />
      <Route path="/watch" element={<Watch />} />
    </Routes>
  );
}
