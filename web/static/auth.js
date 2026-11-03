// Login (Google y enlace por email) y favoritos con Supabase.
// Solo se carga si la web se generó con SUPABASE_URL y SUPABASE_ANON_KEY.
// La clave "anon" es pública: la protección real son las reglas RLS de
// supabase/schema.sql, que limitan cada fila a su propietario.
import { createClient } from "https://cdn.jsdelivr.net/npm/@supabase/supabase-js@2.45.4/+esm";

const cfg = document.querySelector("script[data-supabase-url]").dataset;
const sb = createClient(cfg.supabaseUrl, cfg.supabaseKey, {
  auth: { flowType: "pkce", detectSessionInUrl: true, persistSession: true },
});

const $ = (s, r = document) => r.querySelector(s);
const authBox = $("#auth");
const dlg = $("#authdlg");
let user = null;
let favs = new Set();

const redirectTo = () => location.origin + location.pathname;
const safeUrl = (u, protos = ["https:", "http:"]) => {
  try { return protos.includes(new URL(u).protocol) ? u : ""; } catch { return ""; }
};

// ---------- Cabecera ----------
function renderAuth() {
  $("#saved-link").hidden = !user;
  const old = $(".usermenu", authBox);
  if (old) old.remove();
  const btn = $("[data-auth-open]", authBox);
  btn.hidden = !!user;
  if (!user) return;

  const wrap = document.createElement("div");
  wrap.className = "usermenu";
  const b = document.createElement("button");
  b.type = "button";
  b.className = "avatar";
  b.setAttribute("aria-haspopup", "menu");
  b.setAttribute("aria-expanded", "false");
  b.setAttribute("aria-label", "Account menu");
  const pic = safeUrl(user.user_metadata?.avatar_url || user.user_metadata?.picture || "", ["https:"]);
  if (pic) {
    const img = document.createElement("img");
    img.src = pic; img.alt = ""; img.referrerPolicy = "no-referrer";
    b.append(img);
  } else {
    b.textContent = (user.email || "?")[0].toUpperCase();
  }
  const menu = document.createElement("div");
  menu.className = "menu";
  menu.hidden = true;
  menu.setAttribute("role", "menu");
  const who = document.createElement("div");
  who.className = "menu-who";
  who.textContent = user.user_metadata?.full_name || user.email || "";
  const mail = document.createElement("small");
  mail.textContent = user.email || "";
  who.append(mail);
  const saved = document.createElement("a");
  saved.href = cfg.base + "saved/";
  saved.textContent = "♥ Saved stories";
  saved.setAttribute("role", "menuitem");
  const out = document.createElement("button");
  out.type = "button";
  out.textContent = "Sign out";
  out.setAttribute("role", "menuitem");
  out.onclick = async () => { await sb.auth.signOut(); };
  menu.append(who, saved, out);
  b.onclick = (e) => {
    e.stopPropagation();
    menu.hidden = !menu.hidden;
    b.setAttribute("aria-expanded", String(!menu.hidden));
  };
  wrap.append(b, menu);
  authBox.append(wrap);
}
document.addEventListener("click", () => {
  const m = $(".usermenu .menu");
  if (m) { m.hidden = true; $(".usermenu .avatar").setAttribute("aria-expanded", "false"); }
});

// ---------- Diálogo de acceso ----------
const msg = (t, err) => { const m = $("#authmsg"); m.textContent = t; m.dataset.err = err ? "1" : ""; };
const openDlg = () => { msg(""); dlg.showModal(); };
document.addEventListener("click", (e) => {
  if (e.target.closest("[data-auth-open]")) openDlg();
  if (e.target === dlg) dlg.close();
});
$("[data-provider=google]").onclick = async () => {
  const { error } = await sb.auth.signInWithOAuth({ provider: "google", options: { redirectTo: redirectTo() } });
  if (error) msg(error.message, true);
};
$("#emailform").onsubmit = async (e) => {
  e.preventDefault();
  msg("Sending…");
  const { error } = await sb.auth.signInWithOtp({
    email: $("#authemail").value.trim(),
    options: { emailRedirectTo: redirectTo() },
  });
  msg(error ? error.message : "Check your inbox: we sent you a sign-in link.", !!error);
};

// ---------- Favoritos ----------
const snapshot = (el) => ({
  article_id: el.dataset.id,
  title: el.dataset.title,
  url: el.dataset.url,
  source: el.dataset.source,
  section: el.dataset.section,
  image_url: el.dataset.image || null,
  published: el.dataset.published,
});

async function toggle(el) {
  if (!user) return openDlg();
  const id = el.dataset.id;
  const on = favs.has(id);
  on ? favs.delete(id) : favs.add(id);
  paint();
  const { error } = on
    ? await sb.from("favorites").delete().eq("article_id", id)
    : await sb.from("favorites").insert(snapshot(el));
  if (error && error.code !== "23505") { // 23505 = ya estaba guardado
    on ? favs.add(id) : favs.delete(id);
    paint();
  }
}

function paint() {
  document.querySelectorAll(".card .fav").forEach((b) => {
    const on = favs.has(b.closest(".card").dataset.id);
    b.classList.toggle("on", on);
    b.setAttribute("aria-pressed", String(on));
    b.textContent = on ? "♥" : "♡";
  });
  const big = $("[data-fav-article]");
  if (big) {
    const on = favs.has(big.closest("article").dataset.id);
    big.setAttribute("aria-pressed", String(on));
    big.textContent = on ? "♥ Saved" : "♡ Save";
    big.classList.toggle("on", on);
  }
}

function decorateCards(root = document) {
  root.querySelectorAll(".card[data-id]").forEach((card) => {
    if (card.querySelector(".fav")) return;
    const b = document.createElement("button");
    b.type = "button";
    b.className = "fav";
    b.setAttribute("aria-label", "Save story");
    b.textContent = "♡";
    b.onclick = (e) => { e.preventDefault(); e.stopPropagation(); toggle(card); };
    card.append(b);
  });
}

const big = $("[data-fav-article]");
if (big) big.onclick = () => toggle(big.closest("article"));

async function loadFavs() {
  favs = new Set();
  if (user) {
    const { data } = await sb.from("favorites").select("article_id");
    (data || []).forEach((r) => favs.add(r.article_id));
  }
  paint();
}

// ---------- Página /saved/ ----------
async function renderSaved() {
  const grid = $("#saved-grid");
  if (!grid) return;
  const status = $("#saved-status");
  if (!user) {
    status.textContent = "Sign in to see the stories you saved.";
    grid.replaceChildren();
    return;
  }
  const { data, error } = await sb.from("favorites").select("*").order("created_at", { ascending: false });
  if (error) { status.textContent = "Could not load your saved stories."; return; }
  status.textContent = data.length ? `${data.length} saved ${data.length === 1 ? "story" : "stories"}` : "Nothing saved yet. Tap the ♡ on any story.";
  grid.replaceChildren(...data.map((r) => {
    const a = document.createElement("a");
    a.className = `card sec-${/^[a-z]+$/.test(r.section) ? r.section : "tech"}`;
    a.href = safeUrl(r.url) || "#";
    a.target = "_blank"; a.rel = "noopener noreferrer";
    Object.assign(a.dataset, { id: r.article_id, title: r.title, url: r.url, source: r.source, section: r.section, image: r.image_url || "", published: r.published });
    const th = document.createElement("div"); th.className = "thumb";
    const src = safeUrl(r.image_url || "", ["https:"]);
    if (src) {
      const img = document.createElement("img");
      img.src = src; img.alt = ""; img.loading = "lazy"; img.referrerPolicy = "no-referrer";
      img.onerror = () => img.remove();
      th.append(img);
    }
    const body = document.createElement("div"); body.className = "card-body";
    const h = document.createElement("h3"); h.textContent = r.title;
    const m = document.createElement("span"); m.className = "meta"; m.textContent = r.source;
    body.append(h, m);
    a.append(th, body);
    return a;
  }));
  decorateCards(grid);
  paint();
}

// ---------- Arranque ----------
decorateCards();
sb.auth.onAuthStateChange((_evt, session) => {
  user = session?.user || null;
  // Supabase pide no hacer llamadas a su API dentro del callback.
  setTimeout(async () => {
    renderAuth();
    await loadFavs();
    renderSaved();
    if (user && dlg.open) dlg.close();
  }, 0);
});
