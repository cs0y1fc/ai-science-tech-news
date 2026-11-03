// Buscador de etiquetas: lee tags.json (generado en el build) y sugiere
// etiquetas mientras escribes. Enter o clic lleva a la página de la etiqueta.
(() => {
  const box = document.getElementById("search");
  if (!box) return;
  const base = document.currentScript.dataset.base;
  const btn = box.querySelector(".search-btn");
  const form = box.querySelector(".search-form");
  const input = box.querySelector(".search-input");
  const list = box.querySelector(".search-results");
  let tags = null;
  let active = -1;

  const load = () =>
    tags ||
    (tags = fetch(base + "tags.json")
      .then((r) => (r.ok ? r.json() : []))
      .catch(() => []));

  function open() {
    form.hidden = false;
    btn.setAttribute("aria-expanded", "true");
    input.focus();
    load().then(render);
  }
  function close() {
    form.hidden = true;
    list.hidden = true;
    btn.setAttribute("aria-expanded", "false");
    input.value = "";
    active = -1;
  }

  async function render() {
    const q = input.value.trim().toLowerCase().replace(/^#/, "");
    const all = await load();
    let hits = q
      ? all.filter((x) => x.t.includes(q)).sort((a, b) => (b.t.startsWith(q) - a.t.startsWith(q)) || b.n - a.n)
      : all.slice(0, 8);
    hits = hits.slice(0, 8);
    active = hits.length && q ? 0 : -1;
    list.replaceChildren(
      ...hits.map((x, i) => {
        const li = document.createElement("li");
        li.setAttribute("role", "option");
        const a = document.createElement("a");
        a.href = base + "t/" + encodeURIComponent(x.t) + "/";
        a.textContent = "#" + x.t;
        const n = document.createElement("small");
        n.textContent = x.n;
        a.append(n);
        if (i === active) li.className = "act";
        li.append(a);
        return li;
      })
    );
    if (q && !hits.length) {
      const li = document.createElement("li");
      li.className = "none";
      li.textContent = "No tags match “" + q + "”";
      list.replaceChildren(li);
    }
    list.hidden = !list.children.length;
  }

  function move(d) {
    const items = [...list.querySelectorAll("li:not(.none)")];
    if (!items.length) return;
    active = (active + d + items.length) % items.length;
    items.forEach((li, i) => li.classList.toggle("act", i === active));
  }

  btn.addEventListener("click", () => (form.hidden ? open() : close()));
  input.addEventListener("input", render);
  input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") { e.preventDefault(); move(1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); move(-1); }
    else if (e.key === "Escape") { close(); btn.focus(); }
  });
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const a = list.querySelector("li.act a") || list.querySelector("li a");
    if (a) location.href = a.href;
  });
  document.addEventListener("click", (e) => { if (!box.contains(e.target)) close(); });
  document.addEventListener("keydown", (e) => {
    if (e.key === "/" && !/^(INPUT|TEXTAREA)$/.test(document.activeElement.tagName)) {
      e.preventDefault();
      open();
    }
  });
})();
