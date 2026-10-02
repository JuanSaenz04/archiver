import { test } from "node:test";
import assert from "node:assert/strict";
import { watchJobLogs } from "../src/lib/job-logs.ts";

function setup(t) {
  let source;
  const intervals = new Map();
  class Source extends EventTarget {
    static CLOSED = 2;
    readyState = 1;
    constructor(url) { super(); this.url = url; source = this; }
    close() { this.readyState = Source.CLOSED; }
    emit(type, data) { this.dispatchEvent(new MessageEvent(type, { data: JSON.stringify(data) })); }
  }
  const oldWindow = globalThis.window;
  const oldSource = globalThis.EventSource;
  globalThis.EventSource = Source;
  globalThis.window = {
    setInterval(fn) { intervals.set(1, fn); return 1; },
    clearInterval(id) { intervals.delete(id); },
  };
  t.after(() => { globalThis.window = oldWindow; globalThis.EventSource = oldSource; });
  const lines = [], statuses = [], notices = [];
  let done = 0;
  const close = watchJobLogs("job/id", {
    onLines: batch => lines.push(...batch), onStatus: value => statuses.push(value),
    onNotice: value => notices.push(value), onDone: () => done++,
  });
  return { source, intervals, lines, statuses, notices, close, done: () => done };
}

test("batches lines, ignores replayed IDs, and flushes before completion", t => {
  const s = setup(t);
  assert.equal(s.source.url, "/api/jobs/job%2Fid/logs");
  s.source.emit("logs", [{ id: "1-0", text: "first" }, { id: "2-0", text: "last" }]);
  assert.equal(s.lines.length, 0);
  s.source.emit("logs", [{ id: "1-0" }, { id: "2-0" }, { id: "3-0", text: "new" }]);
  s.source.emit("done", { status: "completed", unavailable: false });
  assert.deepEqual(s.lines.map(line => line.text), ["first", "last", "new"]);
  assert.equal(s.done(), 1);
  assert.equal(s.source.readyState, 2);
  assert.equal(s.intervals.size, 0);
});

test("closing discards pending updates and closes the connection", t => {
  const s = setup(t);
  s.source.emit("logs", [{ id: "1-0", text: "pending" }]);
  s.close();
  s.source.emit("done", { status: "completed" });
  s.source.emit("logs", [{ id: "2-0" }]);
  assert.equal(s.lines.length, 0);
  assert.equal(s.done(), 0);
  assert.equal(s.intervals.size, 0);
  assert.equal(s.source.readyState, 2);
});

test("bounds pending history and reports connection, gap and expired states", t => {
  const s = setup(t);
  s.source.emit("logs", Array.from({ length: 2500 }, (_, i) => ({ id: `${i+1}-0` })));
  s.intervals.get(1)();
  assert.equal(s.lines.length, 2000);
  s.source.onerror();
  assert.match(s.statuses.at(-1), /Reconnecting/);
  s.source.emit("gap", "Older entries unavailable");
  assert.equal(s.notices.at(-1), "Older entries unavailable");
  s.source.emit("done", { status: "failed", unavailable: true });
  assert.match(s.statuses.at(-1), /expired/);
});
