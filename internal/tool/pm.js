(function () {
  "use strict";
  var results = [];
  function fmt(v) { try { return JSON.stringify(v); } catch (e) { return String(v); } }
  function deepEq(a, b) { return fmt(a) === fmt(b); }
  function Assertion(v, neg) { this.v = v; this.neg = !!neg; }
  function check(ok, msg) {
    if (this.neg) { ok = !ok; msg = msg.replace("expected", "expected not"); }
    if (!ok) throw new Error(msg);
    return this;
  }
  var P = Assertion.prototype;
  ["to", "be", "been", "is", "that", "which", "and", "has", "have", "with", "at", "of", "same", "deep"].forEach(function (w) {
    Object.defineProperty(P, w, { get: function () { return this; } });
  });
  Object.defineProperty(P, "not", { get: function () { return new Assertion(this.v, !this.neg); } });
  Object.defineProperty(P, "ok", { get: function () { return check.call(this, !!this.v, "expected " + fmt(this.v) + " to be truthy"); } });
  Object.defineProperty(P, "true", { get: function () { return check.call(this, this.v === true, "expected " + fmt(this.v) + " to be true"); } });
  Object.defineProperty(P, "false", { get: function () { return check.call(this, this.v === false, "expected " + fmt(this.v) + " to be false"); } });
  Object.defineProperty(P, "exist", { get: function () { return check.call(this, this.v !== null && this.v !== undefined, "expected value to exist"); } });
  Object.defineProperty(P, "empty", { get: function () { var n = this.v == null ? 0 : (this.v.length !== undefined ? this.v.length : Object.keys(this.v).length); return check.call(this, n === 0, "expected " + fmt(this.v) + " to be empty"); } });
  P.equal = P.equals = P.eq = function (x) { return check.call(this, this.v === x, "expected " + fmt(this.v) + " to equal " + fmt(x)); };
  P.eql = function (x) { return check.call(this, deepEq(this.v, x), "expected " + fmt(this.v) + " to deeply equal " + fmt(x)); };
  P.a = P.an = function (t) {
    var actual = Array.isArray(this.v) ? "array" : this.v === null ? "null" : typeof this.v;
    return check.call(this, actual === t.toLowerCase(), "expected " + fmt(this.v) + " to be a " + t);
  };
  P.above = P.gt = function (n) { return check.call(this, this.v > n, "expected " + fmt(this.v) + " to be above " + n); };
  P.below = P.lt = function (n) { return check.call(this, this.v < n, "expected " + fmt(this.v) + " to be below " + n); };
  P.least = P.gte = function (n) { return check.call(this, this.v >= n, "expected " + fmt(this.v) + " to be at least " + n); };
  P.most = P.lte = function (n) { return check.call(this, this.v <= n, "expected " + fmt(this.v) + " to be at most " + n); };
  P.property = function (k, val) {
    var has = this.v != null && Object.prototype.hasOwnProperty.call(this.v, k);
    if (arguments.length > 1) return check.call(this, has && deepEq(this.v[k], val), "expected property " + k + " to equal " + fmt(val));
    check.call(this, has, "expected " + fmt(this.v) + " to have property " + k);
    return this.neg ? this : new Assertion(this.v[k]);
  };
  P.lengthOf = P.length = function (n) { var l = this.v == null ? -1 : this.v.length; return check.call(this, l === n, "expected length " + l + " to be " + n); };
  P.include = P.includes = P.contain = function (x) {
    var ok = typeof this.v === "string" || Array.isArray(this.v) ? this.v.indexOf(x) >= 0 : this.v != null && Object.prototype.hasOwnProperty.call(this.v, x);
    return check.call(this, ok, "expected " + fmt(this.v) + " to include " + fmt(x));
  };
  P.keys = function () {
    var ks = Array.prototype.slice.call(arguments); if (ks.length === 1 && Array.isArray(ks[0])) ks = ks[0];
    var have = Object.keys(this.v || {}).sort();
    return check.call(this, deepEq(have, ks.slice().sort()), "expected keys " + fmt(have) + " to be " + fmt(ks));
  };
  P.oneOf = function (list) { return check.call(this, list.indexOf(this.v) >= 0, "expected " + fmt(this.v) + " to be one of " + fmt(list)); };
  P.match = function (re) { return check.call(this, re.test(String(this.v)), "expected " + fmt(this.v) + " to match " + re); };
  globalThis.pm = {
    test: function (name, fn) {
      try { fn(); results.push({ name: String(name), passed: true }); }
      catch (e) { results.push({ name: String(name), passed: false, error: String(e && e.message || e) }); }
    },
    expect: function (v) { return new Assertion(v); },
  };
  globalThis.__results = results;
})();
