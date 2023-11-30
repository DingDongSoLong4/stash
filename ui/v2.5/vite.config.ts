import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import legacy from "@vitejs/plugin-legacy";
import tsconfigPaths from "vite-tsconfig-paths";
import viteCompression from "vite-plugin-compression";

const nolegacy = process.env.VITE_APP_NOLEGACY === "true";
const sourcemap = process.env.VITE_APP_SOURCEMAPS === "true";

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  let plugins = [
    react({
      babel: {
        compact: true,
        plugins: [["graphql-tag", { strip: true }]],
      },
    }),
    legacy({
      modernPolyfills: ["es.string.replace-all"],
      renderLegacyChunks: !nolegacy,
    }),
    tsconfigPaths(),
    viteCompression({
      algorithm: "gzip",
      deleteOriginFile: true,
      threshold: 0,
      filter: /\.(js|json|css|svg|md)$/i,
    }),
  ];

  return {
    base: "",
    build: {
      outDir: "build",
      // minify: "terser",
      sourcemap: sourcemap,
      reportCompressedSize: false,
    },
    define: {
      __DEV__: mode === "development",
    },
    optimizeDeps: {
      entries: "src/index.tsx",
    },
    server: {
      port: 3000,
      cors: false,
    },
    publicDir: "public",
    assetsInclude: ["**/*.md"],
    plugins,
  };
});
