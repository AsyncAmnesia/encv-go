import { defineRspeedy } from "@lynx-js/rspeedy";

export default defineRspeedy({
  source: {
    entry: "./src/App.tsx",
  },
  output: {
    dist: "./dist",
    filename: {
      bundle: "player.lynx.bundle",
    },
  },
});
