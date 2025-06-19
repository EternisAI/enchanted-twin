// electron.vite.config.ts
import { resolve } from "path";
import { defineConfig, externalizeDepsPlugin, loadEnv } from "electron-vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
function inlineEnvVars(prefix, raw) {
  return Object.fromEntries(
    Object.entries(raw).filter(([key]) => key.startsWith(prefix)).map(([key, val]) => [`process.env.${key}`, JSON.stringify(val)])
  );
}
var electron_vite_config_default = defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  return {
    main: {
      plugins: [externalizeDepsPlugin()],
      define: inlineEnvVars("", env)
    },
    preload: {
      plugins: [externalizeDepsPlugin()]
    },
    renderer: {
      base: "./",
      root: "src/renderer",
      resolve: {
        alias: {
          "@renderer": resolve("src/renderer/src")
        }
      },
      plugins: [
        TanStackRouterVite({ target: "react", autoCodeSplitting: true }),
        react(),
        tailwindcss()
      ]
    }
  };
});
export {
  electron_vite_config_default as default
};
