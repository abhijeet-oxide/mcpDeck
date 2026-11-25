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
  globalThis.module = { exports: {} };
  globalThis.exports = globalThis.module.exports;
})();
