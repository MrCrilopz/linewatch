import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/login": "http://localhost:8082",
      "/health": "http://localhost:8082",
      "/dashboard": "http://localhost:8082",
      "/ai": "http://localhost:8082",
      "/meters": "http://localhost:8082",
      "/anomalies": "http://localhost:8082",
    },
  },
});
