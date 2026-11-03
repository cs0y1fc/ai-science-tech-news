// Buscador: lee search.json (generado en el build) y sugiere etiquetas y
// noticias (por título o fuente) mientras escribes. Enter o clic abre el
// resultado marcado.
(() => {
  const box = document.getElementById("search");
  if (!box) return;
  const base = document.currentScript.dataset.base;
  const btn = box.querySelector(".search-btn");
  const form = box.querySelector(".search-form");
  const input = box.querySelector(".search-input");
  const list = box.querySelector(".search-results");
  const MAX_TAGS = 4;
  const MAX_STORIES = 6;
  let data = null;
  let active = -1;

  const load = () =>
    data ||
    (data = fetch(base + "search.json")
      .then((r) => (r.ok ? r.json() : { tags: [], articles: [] }))
      .catch(() => ({ tags: [], articles: [] })));

  function open() {
    form.hidden = false;
    btn.setAttribute("aria-expanded", "true");
    input.focus();
    render();
  }
  function close() {
    form.hidden = true;
    list.hidden = true;
    btn.setAttribute("aria-expanded", "false");
    input.value = "";
    active = -1;
  }

  const heading = (text) => {
    const li = document.createElement("li");
    li.className = "grp";
    li.setAttribute("role", "presentation");
    li.textContent = text;
    return li;
  };

  function option(href, external, label, note) {
    const li = document.createElement("li");
    li.setAttribute("role", "option");
    const a = document.createElement("a");
    a.href = href;
    if (external) { a.target = "_blank"; a.rel = "noopener noreferrer"; }
    const span = document.createElement("span");
    span.textContent = label;
    const small = document.createElement("small");
    small.textContent = note;
    a.append(span, small);
    li.append(a);
    return li;
  }

  async function render() {
    const q = input.value.trim().toLowerCase().replace(/^#/, "");
    const { tags, articles } = await load();
    const words = q.split(/\s+/).filter(Boolean);

    let tagHits = q
      ? tags.filter((x) => x.t.includes(q)).sort((a, b) => (b.t.startsWith(q) - a.t.startsWith(q)) || b.n - a.n)
      : tags.slice(0, 8);
    tagHits = tagHits.slice(0, q ? MAX_TAGS : 8);

    // Cada palabra debe aparecer en el título o en la fuente.
    const storyHits = q
      ? articles
          .filter((a) => {
            const hay = (a.t + " " + a.s).toLowerCase();
            return words.every((w) => hay.includes(w));
          })
          .slice(0, MAX_STORIES)
      : [];

    const items = [];
    if (tagHits.length) {
      items.push(heading("Tags"));
      tagHits.forEach((x) => items.push(option(base + "t/" + encodeURIComponent(x.t) + "/", false, "#" + x.t, x.n)));
    }
    if (storyHits.length) {
      items.push(heading("Stories"));
      storyHits.forEach((a) => {
        const ext = /^https?:/i.test(a.h);
        items.push(option(ext ? a.h : base + a.h, ext, a.t, a.s));
      });
    }
    if (q && !tagHits.length && !storyHits.length) {
      const li = document.createElement("li");
      li.className = "none";
      li.textContent = "No results for “" + q + "”";
      items.push(li);
    }
    list.replaceChildren(...items);
    list.hidden = !items.length;
    active = q && list.querySelector("li[role=option]") ? 0 : -1;
    mark();
  }

  const options = () => [...list.querySelectorAll("li[role=option]")];
  function mark() {
    options().forEach((li, i) => li.classList.toggle("act", i === active));
  }
  function move(d) {
    const n = options().length;
    if (!n) return;
    active = (active + d + n) % n;
    mark();
    options()[active].scrollIntoView({ block: "nearest" });
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
    const li = options()[active] || options()[0];
    const a = li && li.querySelector("a");
    if (!a) return;
    if (a.target === "_blank") window.open(a.href, "_blank", "noopener,noreferrer");
    else location.href = a.href;
  });
  document.addEventListener("click", (e) => { if (!box.contains(e.target)) close(); });
  document.addEventListener("keydown", (e) => {
    if (e.key === "/" && !/^(INPUT|TEXTAREA)$/.test(document.activeElement.tagName)) {
      e.preventDefault();
      open();
    }
  });
})();
