import { runtimeConfig } from "./lib/runtime-config";

const target = document.getElementById("runtime-config");

if (target) {
  target.textContent = JSON.stringify(runtimeConfig, null, 2);
}
