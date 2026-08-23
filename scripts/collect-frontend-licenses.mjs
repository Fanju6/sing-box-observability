#!/usr/bin/env node

import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { promises as fs } from "node:fs";
import path from "node:path";

const LEGAL_FILE = /^(?:licen[cs]e|copying|copyright|notice|patents?)(?:[._-].*)?$/i;
const SKIPPED_DIRECTORIES = new Set([".git", "node_modules"]);
const MAX_LEGAL_FILE_BYTES = 5 * 1024 * 1024;
const LEGAL_OVERRIDES = new Map([
  [
    "react-remove-scroll-bar@2.3.8",
    {
      name: "repository-override/LICENSE",
      // npm 包声明为 MIT，但发布包未携带许可证文件。此文本核对自上游提交：
      // 7301c160fda44cb8cf2b9fdfde61efad35736196
      text: `MIT License

Copyright (c) 2025 Anton Korzunov <thekashey@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`,
    },
  ],
]);

function compareText(left, right) {
  return left < right ? -1 : left > right ? 1 : 0;
}

function usage() {
  return [
    "Usage: node scripts/collect-frontend-licenses.mjs",
    "  --web-root <directory>",
    "  --inventory <FRONTEND-LICENSES.json>",
    "  --notices <FRONTEND-LICENSES.txt>",
  ].join("\n");
}

function parseArguments(argv) {
  const options = {};
  for (let index = 0; index < argv.length; index += 2) {
    const flag = argv[index];
    const value = argv[index + 1];
    if (!flag?.startsWith("--") || value === undefined) {
      throw new Error(usage());
    }
    options[flag.slice(2)] = value;
  }
  for (const required of ["web-root", "inventory", "notices"]) {
    if (!options[required]) throw new Error(`Missing --${required}\n${usage()}`);
  }
  return options;
}

function normalizeText(value) {
  return value.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n").trimEnd() + "\n";
}

function normalizePerson(value) {
  if (typeof value === "string") return value;
  if (!value || typeof value !== "object") return "";
  const name = typeof value.name === "string" ? value.name : "";
  const email = typeof value.email === "string" ? ` <${value.email}>` : "";
  const url = typeof value.url === "string" ? ` (${value.url})` : "";
  return `${name}${email}${url}`.trim();
}

function normalizeRepository(value) {
  if (typeof value === "string") return value;
  if (value && typeof value === "object" && typeof value.url === "string") return value.url;
  return "";
}

function declaredLicense(manifest) {
  if (typeof manifest.license === "string" && manifest.license.trim()) {
    return manifest.license.trim();
  }
  if (Array.isArray(manifest.licenses)) {
    const expressions = manifest.licenses
      .map((item) => (typeof item === "string" ? item : item?.type))
      .filter((item) => typeof item === "string" && item.trim())
      .map((item) => item.trim());
    if (expressions.length > 0) return expressions.join(" OR ");
  }
  return "";
}

async function exists(candidate) {
  try {
    await fs.access(candidate);
    return true;
  } catch {
    return false;
  }
}

async function findPackageRoot(resolvedFile, expectedName) {
  let current = path.dirname(resolvedFile);
  for (;;) {
    const manifestPath = path.join(current, "package.json");
    if (await exists(manifestPath)) {
      const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
      if (manifest.name === expectedName) return await fs.realpath(current);
    }
    const parent = path.dirname(current);
    if (parent === current) break;
    current = parent;
  }
  throw new Error(`Unable to locate package root for ${expectedName} from ${resolvedFile}`);
}

async function resolvePackage(name, fromDirectory) {
  const resolver = createRequire(path.join(fromDirectory, "__license_resolver__.cjs"));
  try {
    const manifestPath = resolver.resolve(`${name}/package.json`);
    return await fs.realpath(path.dirname(manifestPath));
  } catch (manifestError) {
    try {
      const entry = resolver.resolve(name);
      return await findPackageRoot(entry, name);
    } catch (entryError) {
      const error = new Error(`Unable to resolve ${name} from ${fromDirectory}`);
      error.cause = { manifestError, entryError };
      throw error;
    }
  }
}

async function collectLegalFiles(directory, relativeDirectory = "") {
  const entries = await fs.readdir(path.join(directory, relativeDirectory), { withFileTypes: true });
  const files = [];
  entries.sort((left, right) => compareText(left.name, right.name));

  for (const entry of entries) {
    const relativePath = path.join(relativeDirectory, entry.name);
    if (entry.isSymbolicLink()) continue;
    if (entry.isDirectory()) {
      if (!SKIPPED_DIRECTORIES.has(entry.name)) {
        files.push(...(await collectLegalFiles(directory, relativePath)));
      }
      continue;
    }
    if (!entry.isFile() || !LEGAL_FILE.test(entry.name)) continue;

    const absolutePath = path.join(directory, relativePath);
    const stat = await fs.stat(absolutePath);
    if (stat.size > MAX_LEGAL_FILE_BYTES) {
      throw new Error(`Legal file is unexpectedly large: ${absolutePath}`);
    }
    files.push({
      name: relativePath.split(path.sep).join("/"),
      text: normalizeText(await fs.readFile(absolutePath, "utf8")),
    });
  }
  return files;
}

function packageFingerprint(item) {
  const hash = createHash("sha256");
  hash.update(JSON.stringify({
    name: item.name,
    version: item.version,
    license: item.license,
    legalFiles: item.legalFiles,
  }));
  return hash.digest("hex");
}

async function inspectPackage(packageDirectory) {
  const manifestPath = path.join(packageDirectory, "package.json");
  const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
  if (typeof manifest.name !== "string" || typeof manifest.version !== "string") {
    throw new Error(`Package metadata lacks name or version: ${manifestPath}`);
  }
  const license = declaredLicense(manifest);
  if (!license || /^(?:UNLICENSED|UNKNOWN)$/i.test(license)) {
    throw new Error(`${manifest.name}@${manifest.version} has no declared license`);
  }

  let legalFiles = await collectLegalFiles(packageDirectory);
  if (legalFiles.length === 0) {
    const override = LEGAL_OVERRIDES.get(`${manifest.name}@${manifest.version}`);
    if (override) {
      legalFiles = [{ name: override.name, text: normalizeText(override.text) }];
    }
  }
  if (legalFiles.length === 0) {
    throw new Error(
      `${manifest.name}@${manifest.version} has no discoverable LICENSE, NOTICE, ` +
      "COPYING, COPYRIGHT, or PATENTS file",
    );
  }
  legalFiles.sort((left, right) => compareText(left.name, right.name));

  return {
    name: manifest.name,
    version: manifest.version,
    license,
    author: normalizePerson(manifest.author),
    homepage: typeof manifest.homepage === "string" ? manifest.homepage : "",
    repository: normalizeRepository(manifest.repository),
    legalFiles,
    dependencies: manifest.dependencies ?? {},
    optionalDependencies: manifest.optionalDependencies ?? {},
    peerDependencies: manifest.peerDependencies ?? {},
    peerDependenciesMeta: manifest.peerDependenciesMeta ?? {},
    packageDirectory,
  };
}

async function collectProductionPackages(webRoot) {
  const rootManifest = JSON.parse(await fs.readFile(path.join(webRoot, "package.json"), "utf8"));
  const rootDependencies = rootManifest.dependencies ?? {};
  const queue = [];
  for (const name of Object.keys(rootDependencies).sort(compareText)) {
    queue.push({ name, from: webRoot, optional: false });
  }

  const visitedDirectories = new Set();
  const packages = new Map();
  while (queue.length > 0) {
    const next = queue.shift();
    let packageDirectory;
    try {
      packageDirectory = await resolvePackage(next.name, next.from);
    } catch (error) {
      if (next.optional) continue;
      throw error;
    }
    if (visitedDirectories.has(packageDirectory)) continue;
    visitedDirectories.add(packageDirectory);

    const item = await inspectPackage(packageDirectory);
    const key = `${item.name}@${item.version}`;
    const existing = packages.get(key);
    if (existing && packageFingerprint(existing) !== packageFingerprint(item)) {
      throw new Error(`Conflicting legal metadata for ${key}`);
    }
    if (!existing) packages.set(key, item);

    for (const name of Object.keys(item.dependencies).sort(compareText)) {
      queue.push({ name, from: packageDirectory, optional: false });
    }
    for (const name of Object.keys(item.optionalDependencies).sort(compareText)) {
      queue.push({ name, from: packageDirectory, optional: true });
    }
    for (const name of Object.keys(item.peerDependencies).sort(compareText)) {
      const optional = item.peerDependenciesMeta?.[name]?.optional === true;
      // An optional peer installed only for development is not part of the
      // production graph. Required peers are runtime-provided dependencies.
      if (!optional) queue.push({ name, from: packageDirectory, optional: false });
    }
  }

  return [...packages.values()].sort((left, right) =>
    compareText(left.name, right.name) || compareText(left.version, right.version),
  );
}

function makeInventory(packages) {
  return {
    schemaVersion: 1,
    graph: "src/web/package.json 与 src/web/pnpm-lock.yaml 中的生产依赖",
    packages: packages.map((item) => ({
      name: item.name,
      version: item.version,
      license: item.license,
      ...(item.author ? { author: item.author } : {}),
      ...(item.homepage ? { homepage: item.homepage } : {}),
      ...(item.repository ? { repository: item.repository } : {}),
      legalFiles: item.legalFiles.map((file) => file.name),
    })),
  };
}

function makeNotices(packages) {
  const output = [
    "前端第三方许可证汇总",
    "",
    "根据锁定的生产依赖图确定性生成。",
    "以下路径均相对于依赖包，不包含构建机器的本地路径。",
    "",
  ];
  for (const item of packages) {
    output.push("=".repeat(80));
    output.push(`${item.name}@${item.version}`);
    output.push(`Declared license: ${item.license}`);
    if (item.author) output.push(`Author: ${item.author}`);
    if (item.homepage) output.push(`Homepage: ${item.homepage}`);
    if (item.repository) output.push(`Repository: ${item.repository}`);
    output.push("");
    for (const file of item.legalFiles) {
      output.push("-".repeat(80));
      output.push(file.name);
      output.push("-".repeat(80));
      output.push(file.text.trimEnd());
      output.push("");
    }
  }
  return output.join("\n").trimEnd() + "\n";
}

async function writeFile(filename, contents) {
  await fs.mkdir(path.dirname(filename), { recursive: true });
  await fs.writeFile(filename, contents, "utf8");
}

async function main() {
  const options = parseArguments(process.argv.slice(2));
  const webRoot = path.resolve(options["web-root"]);
  const packages = await collectProductionPackages(webRoot);
  if (packages.length === 0) throw new Error("No production frontend dependencies were found");

  await writeFile(
    path.resolve(options.inventory),
    JSON.stringify(makeInventory(packages), null, 2) + "\n",
  );
  await writeFile(path.resolve(options.notices), makeNotices(packages));
  process.stdout.write(`Collected legal notices for ${packages.length} frontend packages.\n`);
}

main().catch((error) => {
  process.stderr.write(`${error.stack ?? error.message}\n`);
  process.exitCode = 1;
});
