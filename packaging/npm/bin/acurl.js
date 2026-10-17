#!/usr/bin/env node
"use strict";
// Runs the acurl binary, downloading it first if postinstall was skipped.

const { spawnSync } = require("node:child_process");
const { install } = require("../lib/install.js");

install()
  .then((bin) => {
    const res = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
    if (res.error) throw res.error;
    process.exit(res.status === null ? 1 : res.status);
  })
  .catch((err) => {
    console.error(`acurl: ${err.message}`);
    process.exit(2);
  });
