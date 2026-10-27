// Sends terminal input to the server in numbered batches. A batch keeps its
// number and its events until the server acknowledges it, so retrying after a
// lost response is seen as a duplicate and input typed meanwhile waits for the
// next batch instead of riding along unseen.
(function (root) {
  // post(seq, events) resolves to {status, next} and rejects on network errors.
  function create(post, opts) {
    opts = opts || {};
    var schedule = opts.schedule || function (fn, ms) { setTimeout(fn, ms); };
    var onEnd = opts.onEnd || function () {};
    var seq = 0, known = false, ready = false, ended = false;
    var pending = [], batch = null, inflight = false;

    function queue(ev) {
      if (ended) return;
      if (ev.type === "resize") pending = pending.filter(function (p) { return p.type !== "resize"; });
      var last = pending[pending.length - 1];
      if (ev.type === "mouse" && ev.action === "motion" && last && last.type === "mouse" && last.action === "motion") pending.pop();
      pending.push(ev);
      flush();
    }

    function stop(text) {
      if (ended) return;
      ended = true;
      onEnd(text);
    }

    async function flush() {
      if (inflight || !ready || !known || ended) return;
      if (!batch) {
        if (!pending.length) return;
        batch = pending;
        pending = [];
      }
      inflight = true;
      try {
        for (;;) {
          var res = await post(seq, batch);
          if (res.status === 409) { seq = res.next; continue; }
          if (res.status === 401 || res.status === 403) { inflight = false; stop("Signed out"); return; }
          if (res.status === 410) { inflight = false; stop("Terminal closed"); return; }
          if (res.status < 200 || res.status > 299) throw new Error("input " + res.status);
          seq = res.next;
          break;
        }
      } catch (err) {
        inflight = false;
        schedule(flush, 500);
        return;
      }
      batch = null;
      inflight = false;
      flush();
    }

    return {
      queue: queue,
      flush: flush,
      // first sets the starting sequence number from the first full frame.
      first: function (n) { if (!known) { seq = n || 1; known = true; } },
      setReady: function (v) { ready = v; if (v) flush(); },
      end: function () { ended = true; },
    };
  }

  var api = { create: create };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.TermSend = api;
})(typeof window !== "undefined" ? window : this);
