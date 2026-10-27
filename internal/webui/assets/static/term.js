(function () {
  var host = document.getElementById("term");
  if (!host) return;
  var prefix = host.dataset.prefix;
  var csrf = (document.querySelector('meta[name="csrf-token"]') || {}).content || "";
  var keys = window.TermKeys;
  var A_BOLD = 1, A_ITALIC = 2, A_UNDER = 4, A_STRIKE = 8, A_WIDE = 16;

  var screen = document.createElement("div");
  screen.className = "term-screen";
  var cursor = document.createElement("div");
  cursor.className = "term-cursor";
  var overlay = document.createElement("div");
  overlay.className = "term-overlay";
  overlay.hidden = true;
  var probe = document.createElement("div");
  probe.className = "term-probe";
  probe.textContent = "W".repeat(50);
  host.append(screen, cursor, overlay, probe);

  var cellW = 8, cellH = 17, rows = 0, cols = 0, lines = [];
  var mouseOn = false, curVisible = true, curRow = 0, curCol = 0;
  var es = null, retry = null, ended = false, resizeTimer = null, lastSize = "";
  var sender = window.TermSend.create(postInput, { onEnd: end });

  function measure() {
    var r = probe.getBoundingClientRect();
    if (r.width > 0 && r.height > 0) { cellW = r.width / 50; cellH = r.height; }
    host.style.setProperty("--cw", cellW + "px");
    host.style.setProperty("--ch", cellH + "px");
  }

  function fit() {
    measure();
    var top = host.getBoundingClientRect().top;
    host.style.height = Math.max(120, window.innerHeight - top) + "px";
    return { cols: Math.max(1, Math.floor(host.clientWidth / cellW)), rows: Math.max(1, Math.floor(host.clientHeight / cellH)) };
  }

  function showOverlay(text) { overlay.textContent = text; overlay.hidden = false; }
  function hideOverlay() { overlay.hidden = true; }

  function renderRow(el, runs) {
    el.replaceChildren();
    for (var i = 0; i < runs.length; i++) {
      var r = runs[i], a = r.a || 0, s = document.createElement("span");
      s.textContent = r.t;
      if (r.f) s.style.color = r.f;
      if (r.b) s.style.backgroundColor = r.b;
      var cls = [];
      if (a & A_BOLD) cls.push("b");
      if (a & A_ITALIC) cls.push("i");
      if (a & A_UNDER) cls.push("u");
      if (a & A_STRIKE) cls.push("s");
      if (a & A_WIDE) cls.push("w");
      if (cls.length) s.className = cls.join(" ");
      el.appendChild(s);
    }
  }

  function placeCursor() {
    cursor.style.display = curVisible && rows ? "block" : "none";
    cursor.style.left = curCol * cellW + "px";
    cursor.style.top = curRow * cellH + "px";
    cursor.style.width = cellW + "px";
    cursor.style.height = cellH + "px";
  }

  function setRows(n) {
    while (lines.length > n) lines.pop().remove();
    while (lines.length < n) {
      var d = document.createElement("div");
      d.className = "term-row";
      screen.appendChild(d);
      lines.push(d);
    }
  }

  function applyFrame(f) {
    if (f.type === "full") {
      rows = f.rows; cols = f.cols;
      host.style.color = f.fg; host.style.backgroundColor = f.bg;
      setRows(rows);
      for (var y = 0; y < rows; y++) renderRow(lines[y], f.lines[y] || []);
      sender.first(f.seq);
    } else if (f.dirty) {
      for (var i = 0; i < f.dirty.length; i++) {
        var u = f.dirty[i];
        if (u.y < lines.length) renderRow(lines[u.y], u.r);
      }
    }
    if (f.cursor) { curRow = f.cursor.r; curCol = f.cursor.c; curVisible = f.cursor.v; }
    if (typeof f.mouse === "boolean") mouseOn = f.mouse;
    if (typeof f.title === "string" && f.title) document.title = f.title;
    placeCursor();
    sender.setReady(true);
  }

  function connect() {
    clearTimeout(retry);
    var size = fit();
    lastSize = size.cols + "x" + size.rows;
    es = new EventSource(prefix + "stream?rows=" + size.rows + "&cols=" + size.cols);
    es.addEventListener("frame", function (e) { hideOverlay(); applyFrame(JSON.parse(e.data)); });
    es.addEventListener("exit", function (e) {
      end("Program exited with status " + JSON.parse(e.data).code);
    });
    es.addEventListener("replaced", function () { end("Terminal opened in another window. Reload to take it back."); });
    es.addEventListener("closed", function (e) { end("Terminal closed: " + (JSON.parse(e.data).reason || "")); });
    es.onerror = function () {
      es.close();
      if (ended) return;
      sender.setReady(false);
      showOverlay("Reconnecting...");
      retry = setTimeout(connect, 1000);
    };
  }

  function end(text) {
    ended = true;
    sender.end();
    if (es) es.close();
    showOverlay(text);
  }

  function send(ev) {
    sender.queue(ev);
  }

  async function postInput(seq, events) {
    var res = await fetch(prefix + "input", {
      method: "POST", credentials: "same-origin",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ seq: seq, events: events }),
    });
    var next = 0;
    try { next = (await res.json()).next; } catch (e) { /* not JSON: an error page */ }
    return { status: res.status, next: next };
  }

  function sendResize() {
    var size = fit();
    var key = size.cols + "x" + size.rows;
    if (key === lastSize) return;
    lastSize = key;
    send({ type: "resize", cols: size.cols, rows: size.rows });
  }
  window.addEventListener("resize", function () {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(sendResize, 120);
  });

  host.addEventListener("keydown", function (e) {
    var ev = keys.toEvent(e);
    if (!ev) return;
    e.preventDefault();
    send(ev);
  });
  host.addEventListener("paste", function (e) {
    var text = e.clipboardData && e.clipboardData.getData("text");
    e.preventDefault();
    if (text) send({ type: "paste", text: text });
  });

  var BUTTONS = ["left", "middle", "right"];
  function cell(e) {
    var r = screen.getBoundingClientRect();
    return {
      x: Math.min(cols - 1, Math.max(0, Math.floor((e.clientX - r.left) / cellW))),
      y: Math.min(rows - 1, Math.max(0, Math.floor((e.clientY - r.top) / cellH))),
    };
  }
  function mods(e) { return (e.shiftKey ? 1 : 0) | (e.altKey ? 2 : 0) | (e.ctrlKey ? 4 : 0); }
  function reportable(e) { return mouseOn && !e.shiftKey; }
  var lastMotion = "";

  host.addEventListener("mousedown", function (e) {
    host.focus();
    if (!reportable(e)) return;
    e.preventDefault();
    var c = cell(e);
    send({ type: "mouse", action: "press", button: BUTTONS[e.button] || "left", x: c.x, y: c.y, mods: mods(e) });
  });
  window.addEventListener("mouseup", function (e) {
    if (!reportable(e)) return;
    var c = cell(e);
    send({ type: "mouse", action: "release", button: BUTTONS[e.button] || "left", x: c.x, y: c.y, mods: mods(e) });
  });
  host.addEventListener("mousemove", function (e) {
    if (!reportable(e)) return;
    var c = cell(e), key = c.x + "," + c.y + "," + e.buttons;
    if (key === lastMotion) return;
    lastMotion = key;
    var button = e.buttons & 1 ? "left" : e.buttons & 4 ? "middle" : e.buttons & 2 ? "right" : "none";
    send({ type: "mouse", action: "motion", button: button, x: c.x, y: c.y, mods: mods(e) });
  });
  host.addEventListener("wheel", function (e) {
    if (!reportable(e)) return;
    e.preventDefault();
    var c = cell(e);
    var button = e.deltaY < 0 ? "wheelup" : e.deltaY > 0 ? "wheeldown" : e.deltaX < 0 ? "wheelleft" : "wheelright";
    send({ type: "mouse", action: "press", button: button, x: c.x, y: c.y, mods: mods(e) });
  }, { passive: false });
  host.addEventListener("contextmenu", function (e) { if (reportable(e)) e.preventDefault(); });
  host.addEventListener("focus", placeCursor);
  host.addEventListener("blur", placeCursor);

  showOverlay("Connecting...");
  connect();
  host.focus();
})();
