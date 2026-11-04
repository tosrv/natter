(function () {
  try {
    let v = null;
    try { v = localStorage.getItem("natter.theme"); } catch { v = null; }
    let dark = v === "dark";
    if (v !== "dark" && v !== "light") {
      dark = typeof window.matchMedia === "function" && window.matchMedia("(prefers-color-scheme: dark)").matches;
    }
    if (dark) document.documentElement.classList.add("dark");
  } catch { /* ignore */ }
})();
