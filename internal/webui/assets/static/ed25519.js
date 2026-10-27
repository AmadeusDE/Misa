// Pure-JS ed25519 (RFC 8032): public key derivation and signing only.
// WebCrypto is unavailable over plain HTTP, so SHA-512 is implemented here too.
var Ed25519 = (function () {
  "use strict";
  var M64 = (1n << 64n) - 1n;

  function iroot(n, k) {
    var lo = 0n, hi = 1n << (BigInt(n.toString(2).length) / BigInt(k) + 1n);
    while (lo < hi) {
      var mid = (lo + hi + 1n) >> 1n;
      if (mid ** BigInt(k) <= n) lo = mid; else hi = mid - 1n;
    }
    return lo;
  }
  function primes(count) {
    var out = [];
    for (var n = 2; out.length < count; n++) {
      if (out.every(function (p) { return n % p !== 0; })) out.push(n);
    }
    return out;
  }
  var pr = primes(80);
  var K = pr.map(function (p) { return iroot(BigInt(p) << 192n, 3) & M64; });
  var H0 = pr.slice(0, 8).map(function (p) { return iroot(BigInt(p) << 128n, 2) & M64; });

  function rotr(x, n) { return ((x >> n) | (x << (64n - n))) & M64; }

  function sha512(msg) {
    var len = msg.length;
    var padded = new Uint8Array(((len + 17 + 127) >> 7) << 7);
    padded.set(msg);
    padded[len] = 0x80;
    var bits = BigInt(len) * 8n;
    for (var i = 0; i < 16; i++) padded[padded.length - 1 - i] = Number((bits >> BigInt(8 * i)) & 0xffn);
    var h = H0.slice();
    var w = new Array(80);
    for (var off = 0; off < padded.length; off += 128) {
      for (var t = 0; t < 16; t++) {
        var v = 0n;
        for (var b = 0; b < 8; b++) v = (v << 8n) | BigInt(padded[off + t * 8 + b]);
        w[t] = v;
      }
      for (t = 16; t < 80; t++) {
        var s0 = rotr(w[t - 15], 1n) ^ rotr(w[t - 15], 8n) ^ (w[t - 15] >> 7n);
        var s1 = rotr(w[t - 2], 19n) ^ rotr(w[t - 2], 61n) ^ (w[t - 2] >> 6n);
        w[t] = (w[t - 16] + s0 + w[t - 7] + s1) & M64;
      }
      var a = h[0], bb = h[1], c = h[2], d = h[3], e = h[4], f = h[5], g = h[6], hh = h[7];
      for (t = 0; t < 80; t++) {
        var S1 = rotr(e, 14n) ^ rotr(e, 18n) ^ rotr(e, 41n);
        var ch = (e & f) ^ (~e & M64 & g);
        var t1 = (hh + S1 + ch + K[t] + w[t]) & M64;
        var S0 = rotr(a, 28n) ^ rotr(a, 34n) ^ rotr(a, 39n);
        var maj = (a & bb) ^ (a & c) ^ (bb & c);
        var t2 = (S0 + maj) & M64;
        hh = g; g = f; f = e; e = (d + t1) & M64; d = c; c = bb; bb = a; a = (t1 + t2) & M64;
      }
      var s = [a, bb, c, d, e, f, g, hh];
      for (i = 0; i < 8; i++) h[i] = (h[i] + s[i]) & M64;
    }
    var out = new Uint8Array(64);
    for (i = 0; i < 8; i++) for (var j = 0; j < 8; j++) out[i * 8 + j] = Number((h[i] >> BigInt(56 - 8 * j)) & 0xffn);
    return out;
  }

  var P = (1n << 255n) - 19n;
  var L = (1n << 252n) + 27742317777372353535851937790883648493n;
  function mod(a, m) { var r = a % (m || P); return r >= 0n ? r : r + (m || P); }
  function pow(b, e, m) {
    var r = 1n; b = mod(b, m);
    while (e > 0n) { if (e & 1n) r = (r * b) % m; b = (b * b) % m; e >>= 1n; }
    return r;
  }
  function inv(a) { return pow(a, P - 2n, P); }
  var D = mod(-121665n * inv(121666n));
  var SQRT_M1 = pow(2n, (P - 1n) / 4n, P);

  function baseX(y) {
    var y2 = mod(y * y);
    var x2 = mod((y2 - 1n) * inv(D * y2 + 1n));
    var x = pow(x2, (P + 3n) / 8n, P);
    if (mod(x * x) !== x2) x = mod(x * SQRT_M1);
    if (x & 1n) x = P - x;
    return x;
  }
  var BY = mod(4n * inv(5n));
  var BX = baseX(BY);
  var BASE = [BX, BY, 1n, mod(BX * BY)];
  var ZERO = [0n, 1n, 1n, 0n];

  function add(p, q) {
    var A = mod((p[1] - p[0]) * (q[1] - q[0]));
    var B = mod((p[1] + p[0]) * (q[1] + q[0]));
    var C = mod(p[3] * 2n * D * q[3]);
    var Dd = mod(p[2] * 2n * q[2]);
    var E = B - A, F = Dd - C, G = Dd + C, H = B + A;
    return [mod(E * F), mod(G * H), mod(F * G), mod(E * H)];
  }
  function mul(p, k) {
    var r = ZERO, q = p;
    while (k > 0n) { if (k & 1n) r = add(r, q); q = add(q, q); k >>= 1n; }
    return r;
  }

  function leToNum(b) { var n = 0n; for (var i = b.length - 1; i >= 0; i--) n = (n << 8n) | BigInt(b[i]); return n; }
  function numToLe(n, len) { var o = new Uint8Array(len); for (var i = 0; i < len; i++) { o[i] = Number(n & 0xffn); n >>= 8n; } return o; }
  function encode(p) {
    var zi = inv(p[2]);
    var x = mod(p[0] * zi), y = mod(p[1] * zi);
    var out = numToLe(y, 32);
    if (x & 1n) out[31] |= 0x80;
    return out;
  }
  function concat() {
    var n = 0, i;
    for (i = 0; i < arguments.length; i++) n += arguments[i].length;
    var out = new Uint8Array(n), o = 0;
    for (i = 0; i < arguments.length; i++) { out.set(arguments[i], o); o += arguments[i].length; }
    return out;
  }

  function expand(seed) {
    var h = sha512(seed);
    var a = h.slice(0, 32);
    a[0] &= 248; a[31] &= 127; a[31] |= 64;
    return { a: leToNum(a), prefix: h.slice(32) };
  }
  function publicKey(seed) { return encode(mul(BASE, expand(seed).a)); }
  function sign(seed, msg) {
    var e = expand(seed);
    var A = encode(mul(BASE, e.a));
    var r = mod(leToNum(sha512(concat(e.prefix, msg))), L);
    var R = encode(mul(BASE, r));
    var k = mod(leToNum(sha512(concat(R, A, msg))), L);
    var S = mod(r + k * e.a, L);
    return concat(R, numToLe(S, 32));
  }

  function toHex(b) { return Array.prototype.map.call(b, function (x) { return x.toString(16).padStart(2, "0"); }).join(""); }
  function fromHex(s) { var o = new Uint8Array(s.length / 2); for (var i = 0; i < o.length; i++) o[i] = parseInt(s.substr(i * 2, 2), 16); return o; }
  function toBase64(b) { var s = ""; for (var i = 0; i < b.length; i++) s += String.fromCharCode(b[i]); return btoa(s); }

  return { sha512: sha512, publicKey: publicKey, sign: sign, toHex: toHex, fromHex: fromHex, toBase64: toBase64 };
})();
if (typeof module !== "undefined") module.exports = Ed25519;
