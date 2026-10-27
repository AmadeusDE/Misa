(function () {
  var meta = document.querySelector('meta[name="csrf-token"]');
  var csrf = meta ? meta.content : "";
  async function post(url, body) {
    var res = await fetch(url, {
      method: "POST", credentials: "same-origin",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: typeof body === "string" ? body : JSON.stringify(body || {}),
    });
    return res;
  }
  window.webui = { csrf: csrf, post: post };
  document.addEventListener("click", async function (e) {
    var t = e.target;
    if (!(t instanceof HTMLElement)) return;
    if (t.matches("[data-logout]")) {
      await post("/auth/logout");
      location.href = "/login";
    } else if (t.matches("[data-dismiss]")) {
      var n = t.closest("[data-notice]");
      await post("/auth/notices/dismiss", { id: n.dataset.notice });
      n.remove();
    } else if (t.dataset.post) {
      var res = await post(t.dataset.post, t.dataset.body || "{}");
      if (res.ok) location.reload();
      else alert(await res.text());
    }
  });
})();
