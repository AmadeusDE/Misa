// Maps DOM keyboard events to the terminal key events the server encodes,
// and decides which keys stay with the browser.
(function (root) {
  var MODIFIER_ONLY = {
    Shift: 1, Control: 1, Alt: 1, Meta: 1, AltGraph: 1, CapsLock: 1, NumLock: 1, ScrollLock: 1,
    Fn: 1, FnLock: 1, OS: 1, Hyper: 1, Super: 1, Symbol: 1, SymbolLock: 1,
    Dead: 1, Process: 1, Unidentified: 1,
  };
  // Left to the browser on purpose: full screen, devtools, and copy/paste/inspect chords.
  var BROWSER_KEYS = { F11: 1, F12: 1 };
  var BROWSER_CTRL_SHIFT = { KeyC: 1, KeyV: 1, KeyI: 1, KeyJ: 1 };

  function codeOf(e) { return e.code || ""; }

  // reserved reports keys left to the browser: Cmd/Win combinations, F11/F12 and
  // the copy, paste and devtools chords.
  function reserved(e) {
    if (e.metaKey) return true;
    if (BROWSER_KEYS[e.key]) return true;
    if (e.ctrlKey && e.shiftKey && !e.altKey && BROWSER_CTRL_SHIFT[codeOf(e)]) return true;
    return false;
  }

  function altGraph(e) { return typeof e.getModifierState === "function" && e.getModifierState("AltGraph"); }

  function plain(e) {
    var ev = {
      type: "key", key: e.key, code: e.code,
      ctrl: !!e.ctrlKey, alt: !!e.altKey, shift: !!e.shiftKey, meta: !!e.metaKey,
    };
    if (altGraph(e)) { ev.ctrl = false; ev.alt = false; }
    if ([...e.key].length === 1 && !ev.ctrl) ev.text = e.key;
    return ev;
  }

  // toEvent returns the event to send for e, or null when the terminal takes no part.
  function toEvent(e) {
    if (e.isComposing || e.keyCode === 229) return null;
    if (MODIFIER_ONLY[e.key] || reserved(e)) return null;
    return plain(e);
  }

  var api = { toEvent: toEvent, reserved: reserved };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.TermKeys = api;
})(typeof window !== "undefined" ? window : this);
