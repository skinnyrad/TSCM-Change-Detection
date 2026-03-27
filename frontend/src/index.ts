import { serve } from "bun";
import index from "./index.html";

const GO_BACKEND = "http://localhost:8080";

const server = serve({
  routes: {
    // Proxy all /api/* requests to the Go backend
    "/api/*": async (req) => {
      const url = new URL(req.url);
      const target = `${GO_BACKEND}${url.pathname}${url.search}`;
      return fetch(target, {
        method: req.method,
        headers: req.headers,
        body: req.body,
      });
    },

    // Serve index.html for all unmatched routes (SPA fallback)
    "/*": index,
  },

  development: process.env.NODE_ENV !== "production" && {
    hmr: true,
    console: true,
  },
});

console.log(`Server running at ${server.url}`);
