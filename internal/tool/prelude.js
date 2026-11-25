(function () {
  "use strict";
  var blocked = function () { throw new Error("eval and Function from strings are turned off in mcpDeck tools"); };
  var protos = [
    Function.prototype,
    Object.getPrototypeOf(async function () {}),
    Object.getPrototypeOf(function* () {}),
  ];
  for (var i = 0; i < protos.length; i++) {
    Object.defineProperty(protos[i], "constructor", { value: blocked, writable: false, configurable: false });
  }
  globalThis.eval = blocked;
  globalThis.Function = blocked;
  var maxRepeat = 4 * 1024 * 1024;
  var repeat = String.prototype.repeat;
  String.prototype.repeat = function (n) {
    if (this.length * n > maxRepeat) throw new RangeError("string too large for an mcpDeck tool");
    return repeat.call(this, n);
  };
  class ApiError extends Error {
    constructor(message, status, body, code) {
      super(message);
      this.name = "ApiError";
      this.status = status;
      this.body = body;
      this.code = code;
    }
  }
  globalThis.ApiError = ApiError;
  // pick(value, ["id", "items[].name", "owner.email"]) keeps only those
  // fields, so a tool returns what an agent needs (FLOW-3).
  function tree(paths) {
    var root = {};
    for (var i = 0; i < paths.length; i++) {
      var segs = String(paths[i]).replace(/\[\]/g, ".[]").split(".");
      var n = root;
      for (var j = 0; j < segs.length; j++) {
        if (segs[j] === "") continue;
        if (!n[segs[j]]) n[segs[j]] = {};
        n = n[segs[j]];
      }
    }
    return root;
  }
  function apply(v, node) {
    var keys = Object.keys(node);
    if (keys.length === 0 || v === null || typeof v !== "object") return v;
    if (Array.isArray(v)) {
      var inner = node["[]"] || node;
      return v.map(function (x) { return apply(x, inner); });
    }
    var out = {};
    for (var i = 0; i < keys.length; i++) {
      var k = keys[i];
      if (k !== "[]" && Object.prototype.hasOwnProperty.call(v, k)) out[k] = apply(v[k], node[k]);
    }
    return out;
  }
  globalThis.pick = function pick(value, paths) {
    if (!Array.isArray(paths) || paths.length === 0) return value;
    return apply(value, tree(paths));
  };
  globalThis.module = { exports: {} };
  globalThis.exports = globalThis.module.exports;
})();
