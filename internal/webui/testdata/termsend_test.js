const S = require(require('path').resolve(process.argv[2], 'termsend.js'));
const assert = require('assert');

const key = (k) => ({ type: 'key', key: k });
const tick = () => new Promise((r) => setImmediate(r));

// A fake server with the same sequence rules as termSession.checkSeq.
function server() {
  const srv = { next: 1, applied: [], requests: [], loseNext: false };
  srv.post = async (seq, events) => {
    srv.requests.push({ seq, events: events.map((e) => e.key) });
    let status = 200;
    if (seq > srv.next) status = 409;
    else if (seq === srv.next) { srv.applied.push(...events.map((e) => e.key)); srv.next++; }
    if (srv.loseNext) { srv.loseNext = false; throw new Error('response lost'); }
    return { status, next: srv.next };
  };
  return srv;
}

(async () => {
  // A response lost after the server applied the batch: the retry resends the
  // same batch with the same number, and keys typed meanwhile still arrive.
  {
    const srv = server();
    const retries = [];
    const s = S.create(srv.post, { schedule: (fn) => retries.push(fn) });
    s.first(1);
    s.setReady(true);
    srv.loseNext = true;
    s.queue(key('a'));
    s.queue(key('b')); // typed while 'a' is in flight
    await tick();
    assert.strictEqual(retries.length, 1, 'one retry scheduled');
    retries.shift()();
    for (let i = 0; i < 5; i++) await tick();
    assert.deepStrictEqual(srv.applied, ['a', 'b']);
    assert.deepStrictEqual(srv.requests[1], { seq: 1, events: ['a'] }, 'the retry is the same batch');
    assert.deepStrictEqual(srv.requests[2], { seq: 2, events: ['b'] });
  }

  // Input before the first frame waits; nothing is sent until ready.
  {
    const srv = server();
    const s = S.create(srv.post, { schedule: () => {} });
    s.queue(key('x'));
    await tick();
    assert.strictEqual(srv.requests.length, 0);
    s.first(1);
    s.setReady(true);
    for (let i = 0; i < 3; i++) await tick();
    assert.deepStrictEqual(srv.applied, ['x']);
  }

  // A signed-out answer ends the sender and reports why.
  {
    let why = '';
    const s = S.create(async () => ({ status: 401, next: 1 }), { onEnd: (t) => { why = t; } });
    s.first(1);
    s.setReady(true);
    s.queue(key('q'));
    for (let i = 0; i < 3; i++) await tick();
    assert.strictEqual(why, 'Signed out');
  }
  console.log('ok');
})().catch((e) => { console.error(e); process.exit(1); });
