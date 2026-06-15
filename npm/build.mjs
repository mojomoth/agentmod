#!/usr/bin/env node
// Build (and optionally publish) the npm packages for agentmod from the
// binaries GoReleaser produced in ./dist.
//
// It reads dist/artifacts.json, emits one platform package per Go binary
// (@agentmod/cli-<os>-<arch>, carrying that binary and an os/cpu constraint)
// plus a version-stamped copy of the `agentmod` launcher package, into
// ./npm/dist. With --publish it runs `npm publish --access public` on each,
// platform packages first so the launcher's optionalDependencies resolve.
//
// Usage:
//   AGENTMOD_VERSION=v1.2.3 node npm/build.mjs            # stage only (dry run)
//   AGENTMOD_VERSION=v1.2.3 node npm/build.mjs --publish  # stage + publish
//
// Env:
//   AGENTMOD_VERSION  release version (a leading "v" is stripped). Required
//                     with --publish.
//   AGENTMOD_DIST     GoReleaser dist directory (default ./dist).

import { execFileSync } from "node:child_process";
import {
  chmodSync,
  copyFileSync,
  cpSync,
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const SCOPE = "@agentmod";
const PLATFORM_PREFIX = `${SCOPE}/cli`;
const MAIN_PKG = "agentmod";

const here = dirname(fileURLToPath(import.meta.url)); // <repo>/npm
const repoRoot = resolve(here, "..");
const distDir = resolve(repoRoot, process.env.AGENTMOD_DIST || "dist");
const outDir = resolve(here, "dist"); // <repo>/npm/dist (generated, gitignored)
const srcMain = resolve(here, MAIN_PKG); // <repo>/npm/agentmod

const publish = process.argv.includes("--publish");

// GoReleaser GOOS/GOARCH -> Node process.platform/process.arch.
const OS_MAP = { linux: "linux", darwin: "darwin", windows: "win32" };
const ARCH_MAP = { amd64: "x64", arm64: "arm64" };

function fail(msg) {
  console.error(`build.mjs: ${msg}`);
  process.exit(1);
}

function resolveVersion() {
  const v = (process.env.AGENTMOD_VERSION || "").replace(/^v/, "").trim();
  if (!v) {
    if (publish) fail("AGENTMOD_VERSION is required for --publish");
    return "0.0.0-dev";
  }
  return v;
}

function writeJSON(path, obj) {
  writeFileSync(path, JSON.stringify(obj, null, 2) + "\n");
}

function readBinaries() {
  const af = join(distDir, "artifacts.json");
  if (!existsSync(af)) {
    fail(`${af} not found — run goreleaser first (it writes dist/artifacts.json)`);
  }
  const artifacts = JSON.parse(readFileSync(af, "utf8"));
  const bins = artifacts
    .filter((a) => a.type === "Binary")
    .map((a) => {
      const platform = OS_MAP[a.goos];
      const arch = ARCH_MAP[a.goarch];
      if (!platform || !arch) return null;
      return { platform, arch, path: resolve(repoRoot, a.path) };
    })
    .filter(Boolean);
  if (bins.length === 0) fail("no usable Binary artifacts in artifacts.json");
  return bins;
}

function buildPlatformPackage(bin, version) {
  const name = `${PLATFORM_PREFIX}-${bin.platform}-${bin.arch}`;
  const pkgDir = join(outDir, `cli-${bin.platform}-${bin.arch}`);
  const binDir = join(pkgDir, "bin");
  mkdirSync(binDir, { recursive: true });
  const binName = bin.platform === "win32" ? "agentmod.exe" : "agentmod";
  const dest = join(binDir, binName);
  copyFileSync(bin.path, dest);
  if (bin.platform !== "win32") chmodSync(dest, 0o755);
  writeJSON(join(pkgDir, "package.json"), {
    name,
    version,
    description: `agentmod prebuilt binary for ${bin.platform}-${bin.arch}`,
    repository: { type: "git", url: "git+https://github.com/mojomoth/agentmod.git" },
    license: "MIT",
    os: [bin.platform],
    cpu: [bin.arch],
    files: [`bin/${binName}`],
  });
  return { name, pkgDir };
}

function buildMainPackage(version, platformNames) {
  const pkgDir = join(outDir, MAIN_PKG);
  cpSync(srcMain, pkgDir, { recursive: true });
  const pkgJsonPath = join(pkgDir, "package.json");
  const pkg = JSON.parse(readFileSync(pkgJsonPath, "utf8"));
  pkg.version = version;
  pkg.optionalDependencies = {};
  for (const n of platformNames.sort()) pkg.optionalDependencies[n] = version;
  writeJSON(pkgJsonPath, pkg);
  return { name: MAIN_PKG, pkgDir };
}

function npmPublish(pkgDir) {
  execFileSync("npm", ["publish", "--access", "public"], {
    cwd: pkgDir,
    stdio: "inherit",
  });
}

function main() {
  const version = resolveVersion();
  rmSync(outDir, { recursive: true, force: true });
  mkdirSync(outDir, { recursive: true });

  const platformPkgs = readBinaries().map((b) => buildPlatformPackage(b, version));
  const mainPkg = buildMainPackage(version, platformPkgs.map((p) => p.name));
  console.log(
    `Staged ${platformPkgs.length} platform packages + ${MAIN_PKG}@${version} in ${outDir}`
  );

  if (!publish) {
    console.log("dry run (pass --publish to publish)");
    return;
  }
  for (const p of platformPkgs) {
    console.log(`publishing ${p.name}@${version}`);
    npmPublish(p.pkgDir);
  }
  console.log(`publishing ${mainPkg.name}@${version}`);
  npmPublish(mainPkg.pkgDir);
  console.log("npm publish complete");
}

main();
