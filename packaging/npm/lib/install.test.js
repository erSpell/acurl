"use strict";
const test = require("node:test");
const assert = require("node:assert");
const { target, expectedSum, VERSION } = require("./install.js");

test("maps platforms to release archives", () => {
  assert.deepStrictEqual(target("darwin", "arm64"), { archive: `acurl_${VERSION}_darwin_arm64.tar.gz`, binary: "acurl" });
  assert.deepStrictEqual(target("win32", "x64"), { archive: `acurl_${VERSION}_windows_amd64.zip`, binary: "acurl.exe" });
  assert.throws(() => target("aix", "ppc64"), /no prebuilt binary/);
});

test("finds the expected checksum", () => {
  const sums = "aaa  acurl_1.0.0_linux_amd64.tar.gz\nbbb  acurl_1.0.0_darwin_arm64.tar.gz\n";
  assert.strictEqual(expectedSum(sums, "acurl_1.0.0_darwin_arm64.tar.gz"), "bbb");
  assert.throws(() => expectedSum(sums, "nope.tar.gz"), /not listed/);
});
