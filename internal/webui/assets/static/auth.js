// Browser side of enrollment and login. The seed lives in IndexedDB; signing
// uses the pure-JS ed25519 because WebCrypto is unavailable without HTTPS.
(function () {
  var status = document.getElementById("status");
  function say(msg) { if (status) status.textContent = msg; }

  function idb() {
    return new Promise(function (resolve, reject) {
      var req = indexedDB.open("webui", 1);
      req.onupgradeneeded = function () { req.result.createObjectStore("keys"); };
      req.onsuccess = function () { resolve(req.result); };
      req.onerror = function () { reject(req.error); };
    });
  }
  async function loadKey() {
    var db = await idb();
    return new Promise(function (resolve, reject) {
      var r = db.transaction("keys").objectStore("keys").get("device");
      r.onsuccess = function () { resolve(r.result || null); };
      r.onerror = function () { reject(r.error); };
    });
  }
  async function saveKey(rec) {
    var db = await idb();
    return new Promise(function (resolve, reject) {
      var tx = db.transaction("keys", "readwrite");
      tx.objectStore("keys").put(rec, "device");
      tx.oncomplete = resolve;
      tx.onerror = function () { reject(tx.error); };
    });
  }
  async function postJSON(url, body) {
    var res = await fetch(url, { method: "POST", credentials: "same-origin",
      headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    var data = await res.json().catch(function () { return {}; });
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
  }

  async function login(rec, next) {
    var ch = await (await fetch("/auth/challenge", { credentials: "same-origin" })).json();
    var msg = new TextEncoder().encode("webui-login-v1:" + ch.nonce);
    var sig = Ed25519.sign(rec.seed, msg);
    await postJSON("/auth/login", { device_id: rec.deviceId, nonce: ch.nonce, signature: Ed25519.toBase64(sig) });
    location.href = next || "/";
  }

  var loginEl = document.getElementById("login");
  if (loginEl) {
    loadKey().then(function (rec) {
      if (!rec) { say("This browser has no device key. Enroll it first."); return; }
      return login(rec, loginEl.dataset.next);
    }).catch(function (e) { say("Sign in failed: " + e.message); });
  }

  var form = document.getElementById("enroll");
  if (form) {
    form.addEventListener("submit", async function (ev) {
      ev.preventDefault();
      try {
        var seed = crypto.getRandomValues(new Uint8Array(32));
        var pub = Ed25519.publicKey(seed);
        var res = await postJSON("/auth/enroll", { code: form.code.value, name: form.name.value, pubkey: Ed25519.toBase64(pub) });
        await saveKey({ seed: seed, deviceId: res.device_id });
        say("Enrolled. Signing in...");
        await login({ seed: seed, deviceId: res.device_id }, "/");
      } catch (e) { say("Enrollment failed: " + e.message); }
    });
  }
})();
