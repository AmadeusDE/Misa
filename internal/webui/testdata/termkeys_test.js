const K = require(require('path').resolve(process.argv[2], 'termkeys.js'));
const assert = require('assert');
const ke = (o) => Object.assign({ key: '', code: '', ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, isComposing: false, keyCode: 0 }, o);
const letterCode = (l) => 'Key' + l.toUpperCase();
const fields = (e) => e && [e.key, e.ctrl, e.alt, e.shift, e.text];

let e = K.toEvent(ke({ key: 'a', code: 'KeyA' }));
assert.deepStrictEqual([e.type, e.key, e.code, e.text, e.ctrl], ['key', 'a', 'KeyA', 'a', false]);

// Every ctrl+letter and ctrl+shift+letter is forwarded, except the copy/paste/devtools chords.
for (const l of 'abcdefghijklmnopqrstuvwxyz') {
  e = K.toEvent(ke({ key: l, code: letterCode(l), ctrlKey: true }));
  assert(e && e.ctrl && e.text === undefined, 'ctrl+' + l);
  e = K.toEvent(ke({ key: l.toUpperCase(), code: letterCode(l), ctrlKey: true, shiftKey: true }));
  if ('cvij'.includes(l)) assert.strictEqual(e, null, 'ctrl+shift+' + l + ' stays with the browser');
  else assert(e && e.ctrl && e.shift, 'ctrl+shift+' + l);
  e = K.toEvent(ke({ key: l, code: letterCode(l), altKey: true }));
  assert(e && e.alt && !e.ctrl, 'alt+' + l);
}
// Chords the browser keeps are forwarded when they do reach the page (Keyboard Lock, localhost).
for (const o of [{ key: 'Tab', ctrlKey: true }, { key: 'Tab', ctrlKey: true, shiftKey: true },
  { key: 'PageUp', ctrlKey: true }, { key: 'PageDown', ctrlKey: true }, { key: '1', code: 'Digit1', ctrlKey: true }]) {
  assert(K.toEvent(ke(o)), JSON.stringify(o));
}
for (const k of ['[', ']', '\\', '/', ' ', '-']) assert(K.toEvent(ke({ key: k, ctrlKey: true })), 'ctrl+' + k);
for (let i = 1; i <= 10; i++) assert(K.toEvent(ke({ key: 'F' + i })), 'F' + i);
for (const k of ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'Backspace']) {
  e = K.toEvent(ke({ key: k, altKey: true }));
  assert(e && e.alt && e.text === undefined, 'alt+' + k);
}
assert.strictEqual(K.toEvent(ke({ key: 'F5' })).key, 'F5');

// Left to the browser.
for (const o of [{ key: 'F11' }, { key: 'F12' }, { key: 'c', code: 'KeyC', metaKey: true },
  { key: 'C', code: 'KeyC', ctrlKey: true, shiftKey: true }, { key: 'V', code: 'KeyV', ctrlKey: true, shiftKey: true },
  { key: 'I', code: 'KeyI', ctrlKey: true, shiftKey: true }, { key: 'J', code: 'KeyJ', ctrlKey: true, shiftKey: true },
  { key: 'Shift' }, { key: 'a', isComposing: true }, { key: 'Process', keyCode: 229 }]) {
  assert.strictEqual(K.toEvent(ke(o)), null, JSON.stringify(o));
}
assert.strictEqual(K.toEvent(ke({ key: '👍' })).text, '👍');

// AltGr (reported as ctrl+alt) must not turn into a chord.
e = K.toEvent(ke({ key: '@', code: 'KeyQ', ctrlKey: true, altKey: true, getModifierState: (m) => m === 'AltGraph' }));
assert(!e.ctrl && !e.alt && e.text === '@');

