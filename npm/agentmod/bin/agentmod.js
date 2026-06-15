#!/usr/bin/env node
"use strict";

// Launcher for the agentmod CLI installed via npm.
//
// The actual Go binary is not in this package. It ships in a platform-specific
// optional dependency (@agentmod/cli-<os>-<arch>) so that `npm install` pulls
// down only the one binary matching the host — the esbuild distribution model,
// with no postinstall network download. This launcher resolves that binary for
// the current platform and execs it, forwarding argv and the exit code.

const { execFileSync } = require("node:child_process");

const SCOPE = "@agentmod";

function resolveBinary() {
  const platform = process.platform; // "linux" | "darwin" | "win32" | ...
  const arch = process.arch; // "x64" | "arm64" | ...
  const pkg = `${SCOPE}/cli-${platform}-${arch}`;
  const binName = platform === "win32" ? "agentmod.exe" : "agentmod";
  try {
    return require.resolve(`${pkg}/bin/${binName}`);
  } catch {
    return null;
  }
}

const bin = resolveBinary();
if (!bin) {
  process.stderr.write(
    `agentmod: no prebuilt binary available for ${process.platform}-${process.arch}.\n` +
      `The matching package (${SCOPE}/cli-${process.platform}-${process.arch}) was not installed.\n` +
      `This usually means the platform is unsupported or install ran with --no-optional / --omit=optional.\n` +
      `Alternatives:\n` +
      `  brew install mojomoth/tap/agentmod\n` +
      `  curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh\n` +
      `  go install github.com/mojomoth/agentmod@latest\n`
  );
  process.exit(1);
}

try {
  execFileSync(bin, process.argv.slice(2), { stdio: "inherit" });
} catch (err) {
  // Non-zero exit from the child: mirror its status. Anything else: report.
  if (err && typeof err.status === "number") process.exit(err.status);
  if (err && err.signal) process.exit(1);
  process.stderr.write(`agentmod: failed to run ${bin}: ${err && err.message}\n`);
  process.exit(1);
}
