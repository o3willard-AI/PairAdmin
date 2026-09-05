import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      reporter: ["json", "text"],
      // Emit coverage for source only (not config/test/build artifacts), so the
      // diff-coverage gate intersects git diff ranges with real statements.
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/test/**",
        "**/*.test.ts",
        "**/*.test.tsx",
        "**/__tests__/**",
      ],
    },
  },
});
