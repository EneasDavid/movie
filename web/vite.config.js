import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Build output goes straight into the Go server's embedded static
// directory (internal/web/static) — `go:embed` picks up whatever is
// there at `go build` time, so `npm run build` must run before `go
// build`/`go run` for changes to show up. See the root README and
// Makefile for the combined build command.
export default defineConfig({
  plugins: [react()],
  root: __dirname,
  base: "/",
  build: {
    outDir: "../internal/web/static",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      // During `npm run dev`, forward API calls to the Go server so the
      // whole stack can run locally without a combined build.
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
