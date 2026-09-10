import { spawn, spawnSync } from "node:child_process";
import { closeSync, existsSync, mkdirSync, openSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import http from "node:http";
import net from "node:net";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const runtimeDir = path.join(rootDir, ".runtime");
const backendDir = path.join(rootDir, "backend");
const backendBinary = path.join(
  rootDir,
  "desktop",
  "bin",
  process.platform === "win32" ? "easy-stock-backend.exe" : "easy-stock-backend",
);
const viteEntry = path.join(rootDir, "node_modules", "vite", "bin", "vite.js");

const backendAddress = process.env.A_STOCK_ADDR || "127.0.0.1:20081";
const frontendHost = process.env.A_STOCK_FRONTEND_HOST || "127.0.0.1";
const frontendPort = parsePort(process.env.A_STOCK_FRONTEND_PORT || "20073", "A_STOCK_FRONTEND_PORT");
const { host: backendHost, port: backendPort } = parseAddress(backendAddress);
const backendUrl = `http://${urlHost(backendHost)}:${backendPort}`;
const frontendUrl = `http://${urlHost(frontendHost)}:${frontendPort}`;

const files = {
  backendPid: path.join(runtimeDir, "backend.pid"),
  frontendPid: path.join(runtimeDir, "frontend.pid"),
  backendLog: path.join(runtimeDir, "backend.log"),
  frontendLog: path.join(runtimeDir, "frontend.log"),
};

export const DEFAULT_BROWSER_MODE = "incognito";
export const ALLOWED_BROWSER_MODES = new Set(["incognito", "normal", "none"]);

export function resolveBrowserMode(value = process.env.A_STOCK_BROWSER_MODE) {
  if (value === undefined || value === null) {
    return DEFAULT_BROWSER_MODE;
  }
  const normalized = String(value).trim().toLowerCase();
  if (!normalized) {
    return DEFAULT_BROWSER_MODE;
  }
  if (ALLOWED_BROWSER_MODES.has(normalized)) {
    return normalized;
  }
  return DEFAULT_BROWSER_MODE;
}

export function getBrowserLaunchPlan(targetUrl, mode = resolveBrowserMode()) {
  if (mode === "none") {
    return { action: "none", mode };
  }
  const args = [];
  if (mode === "incognito") {
    args.push("--incognito");
  }
  args.push("--new-window", targetUrl);
  return { action: "open", mode, args };
}

export function findChromeBinary({
  env = process.env,
  existsSync: exists = existsSync,
  whereLookup = () => capture("where.exe", ["chrome.exe"]),
} = {}) {
  const candidates = [];
  if (process.platform === "win32") {
    if (env.ProgramFiles) {
      candidates.push(path.join(env.ProgramFiles, "Google", "Chrome", "Application", "chrome.exe"));
    }
    if (env["ProgramFiles(x86)"]) {
      candidates.push(path.join(env["ProgramFiles(x86)"], "Google", "Chrome", "Application", "chrome.exe"));
    }
    if (env.LOCALAPPDATA) {
      candidates.push(path.join(env.LOCALAPPDATA, "Google", "Chrome", "Application", "chrome.exe"));
    }
  }

  for (const candidate of candidates) {
    if (candidate && exists(candidate)) {
      return candidate;
    }
  }

  if (process.platform === "win32") {
    try {
      const whereResult = whereLookup();
      if (whereResult) {
        const first = whereResult.split(/\r?\n/)[0]?.trim();
        if (first && exists(first)) {
          return first;
        }
      }
    } catch {
      // ignore lookup failures
    }
  }

  return null;
}

export function launchBrowser({
  url = frontendUrl,
  mode = resolveBrowserMode(),
  logger = log,
  chromeFinder = findChromeBinary,
  spawner = spawn,
} = {}) {
  const plan = getBrowserLaunchPlan(url, mode);
  if (plan.action === "none") {
    logger("browser auto-open disabled");
    return { opened: false, reason: "disabled" };
  }

  const chromePath = chromeFinder();
  if (!chromePath) {
    logger(`Chrome not found; open manually:\n${url}`);
    return { opened: false, reason: "not_found" };
  }

  if (plan.mode === "incognito") {
    logger("opening frontend in Chrome incognito mode");
    logger(`frontend: ${url}`);
  } else {
    logger("opening frontend in Chrome normal mode");
    logger(`frontend: ${url}`);
  }

  try {
    const child = spawner(chromePath, plan.args, {
      detached: true,
      stdio: "ignore",
      windowsHide: false,
    });
    if (child?.unref) {
      child.unref();
    }
    return { opened: true, mode: plan.mode };
  } catch (error) {
    logger(`Failed to open browser (${error.message}); open manually:\n${url}`);
    return { opened: false, reason: "error", error };
  }
}

function log(message) {
  console.log(`[easy-stock] ${message}`);
}

function parsePort(value, name) {
  const port = Number(value);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`${name} must be an integer between 1 and 65535.`);
  }
  return port;
}

function parseAddress(value) {
  const bracketed = /^\[([^\]]+)]:(\d+)$/.exec(value);
  if (bracketed) {
    return { host: bracketed[1], port: parsePort(bracketed[2], "A_STOCK_ADDR port") };
  }
  const separator = value.lastIndexOf(":");
  if (separator <= 0) {
    throw new Error("A_STOCK_ADDR must use host:port format.");
  }
  return {
    host: value.slice(0, separator),
    port: parsePort(value.slice(separator + 1), "A_STOCK_ADDR port"),
  };
}

function urlHost(host) {
  return host.includes(":") ? `[${host}]` : host;
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: rootDir,
    env: process.env,
    stdio: "inherit",
    ...options,
  });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`${command} exited with status ${result.status}`);
  }
}

function runNpm(args) {
  if (process.env.npm_execpath) {
    run(process.execPath, [process.env.npm_execpath, ...args]);
    return;
  }
  const npm = process.platform === "win32" ? "npm.cmd" : "npm";
  run(npm, args, { shell: process.platform === "win32" });
}

function buildBackend() {
  mkdirSync(path.dirname(backendBinary), { recursive: true });
  log("building backend");
  run("go", ["build", "-o", backendBinary, "./cmd/server"], { cwd: backendDir });
}

function buildFrontend() {
  log("building frontend");
  runNpm(["--workspace", "frontend", "run", "build"]);
}

function processExists(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return error?.code === "EPERM";
  }
}

function readPidMetadata(pidFile, role) {
  if (!existsSync(pidFile)) return null;
  const raw = readFileSync(pidFile, "utf8").trim();
  if (!raw) {
    rmSync(pidFile, { force: true });
    return null;
  }
  try {
    const parsed = JSON.parse(raw);
    return { ...parsed, pid: Number(parsed.pid), legacy: false };
  } catch {
    const pid = Number(raw);
    if (!Number.isInteger(pid) || pid <= 0) {
      throw new Error(`${role} PID file is invalid: ${pidFile}`);
    }
    return { pid, role, root: rootDir, legacy: true };
  }
}

function capture(command, args) {
  const result = spawnSync(command, args, { encoding: "utf8", windowsHide: true });
  if (result.error || result.status !== 0) return "";
  return result.stdout.trim();
}

function commandLineFor(pid) {
  if (process.platform === "linux") {
    try {
      return readFileSync(`/proc/${pid}/cmdline`, "utf8").replaceAll("\0", " ").trim();
    } catch {
      return "";
    }
  }
  return capture("ps", ["-p", String(pid), "-o", "command="]);
}

function windowsCommandMatches(pid, expected) {
  const escaped = normalizeCommand(expected).replaceAll("'", "''");
  const script = [
    `$item = Get-CimInstance Win32_Process -Filter 'ProcessId=${pid}' -ErrorAction SilentlyContinue`,
    `$expected = '${escaped}'`,
    "$actual = if ($item) { $item.CommandLine.Replace('\\', '/').ToLowerInvariant() } else { '' }",
    "if ($actual.Contains($expected)) { 'MATCH' } else { 'NO_MATCH' }",
  ].join("; ");
  const encoded = Buffer.from(script, "utf16le").toString("base64");
  return capture("powershell.exe", ["-NoProfile", "-NonInteractive", "-EncodedCommand", encoded]) === "MATCH";
}

function normalizeCommand(value) {
  return value.replaceAll("\\", "/").toLowerCase();
}

function isManagedProcess(metadata, role) {
  if (metadata.role && metadata.role !== role) return false;
  if (metadata.root && normalizeCommand(path.resolve(metadata.root)) !== normalizeCommand(rootDir)) return false;
  const expected = normalizeCommand(role === "backend" ? backendBinary : viteEntry);
  if (process.platform === "win32") return windowsCommandMatches(metadata.pid, expected);
  const commandLine = normalizeCommand(commandLineFor(metadata.pid));
  if (!commandLine) return false;
  return commandLine.includes(expected);
}

function pause(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function waitForExit(pid, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (!processExists(pid)) return true;
    await pause(200);
  }
  return !processExists(pid);
}

function signalProcessTree(pid, force) {
  if (process.platform === "win32") {
    const args = ["/PID", String(pid), "/T"];
    if (force) args.push("/F");
    spawnSync("taskkill.exe", args, { stdio: "ignore", windowsHide: true });
    return;
  }
  try {
    process.kill(-pid, force ? "SIGKILL" : "SIGTERM");
  } catch (error) {
    if (error?.code !== "ESRCH") throw error;
  }
}

async function stopManagedProcess(pidFile, role) {
  const metadata = readPidMetadata(pidFile, role);
  if (!metadata) return;
  if (!processExists(metadata.pid)) {
    log(`removing stale ${role} PID metadata`);
    rmSync(pidFile, { force: true });
    return;
  }
  if (!isManagedProcess(metadata, role)) {
    throw new Error(
      `Refusing to stop PID ${metadata.pid}: it cannot be verified as this workspace's ${role} process.`,
    );
  }
  log(`stopping managed ${role} pid=${metadata.pid}`);
  signalProcessTree(metadata.pid, false);
  if (!(await waitForExit(metadata.pid, 4000))) {
    log(`force stopping managed ${role} pid=${metadata.pid}`);
    signalProcessTree(metadata.pid, true);
    if (!(await waitForExit(metadata.pid, 4000))) {
      throw new Error(`Managed ${role} process ${metadata.pid} did not stop.`);
    }
  }
  rmSync(pidFile, { force: true });
}

function portIsFree(host, port) {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.once("error", (error) => {
      if (error?.code === "EADDRINUSE" || error?.code === "EACCES") {
        resolve(false);
      } else {
        reject(error);
      }
    });
    server.listen({ host, port, exclusive: true }, () => server.close(() => resolve(true)));
  });
}

async function requireFreePort(host, port) {
  if (!(await portIsFree(host, port))) {
    throw new Error(`Port ${port} is already in use by an unmanaged process. No process was terminated.`);
  }
}

function writePidMetadata(pidFile, role, child, command) {
  writeFileSync(
    pidFile,
    `${JSON.stringify({ pid: child.pid, role, root: rootDir, command, startedAt: new Date().toISOString() }, null, 2)}\n`,
    "utf8",
  );
}

function startDetached({ role, command, args, cwd, env, logFile, pidFile }) {
  const output = openSync(logFile, "a");
  let child;
  try {
    child = spawn(command, args, {
      cwd,
      env,
      detached: true,
      windowsHide: true,
      stdio: ["ignore", output, output],
    });
  } finally {
    closeSync(output);
  }
  child.once("error", (error) => console.error(`[easy-stock] ${role} launch error: ${error.message}`));
  child.unref();
  writePidMetadata(pidFile, role, child, command);
  return child.pid;
}

function httpReady(url, headers = {}) {
  return new Promise((resolve) => {
    const request = http.get(url, { headers, timeout: 1500 }, (response) => {
      response.resume();
      resolve(response.statusCode >= 200 && response.statusCode < 400);
    });
    request.on("timeout", () => request.destroy());
    request.on("error", () => resolve(false));
  });
}

async function waitForHttp(url, role, pid, headers = {}) {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    if (!processExists(pid)) {
      throw new Error(`${role} exited before becoming ready. Inspect ${path.join(runtimeDir, `${role}.log`)}`);
    }
    if (await httpReady(url, headers)) {
      log(`${role} ready: ${url}`);
      return;
    }
    await pause(500);
  }
  throw new Error(`${role} did not become ready: ${url}`);
}

async function restart() {
  mkdirSync(runtimeDir, { recursive: true });
  if (!existsSync(path.join(rootDir, "node_modules"))) {
    log("node_modules missing, running npm ci --ignore-scripts");
    runNpm(["ci", "--ignore-scripts"]);
  }

  buildBackend();
  buildFrontend();

  await stopManagedProcess(files.backendPid, "backend");
  await stopManagedProcess(files.frontendPid, "frontend");
  await requireFreePort(backendHost, backendPort);
  await requireFreePort(frontendHost, frontendPort);

  log(`starting backend on ${backendAddress}`);
  const backendPid = startDetached({
    role: "backend",
    command: backendBinary,
    args: [],
    cwd: rootDir,
    env: { ...process.env, A_STOCK_ADDR: backendAddress, A_STOCK_TOKEN: process.env.A_STOCK_TOKEN || "" },
    logFile: files.backendLog,
    pidFile: files.backendPid,
  });
  await waitForHttp(`${backendUrl}/api/health`, "backend", backendPid);

  log(`starting frontend on ${frontendUrl}`);
  const frontendPid = startDetached({
    role: "frontend",
    command: process.execPath,
    args: [viteEntry, "--host", frontendHost, "--port", String(frontendPort), "--strictPort"],
    cwd: path.join(rootDir, "frontend"),
    env: {
      ...process.env,
      VITE_A_STOCK_BACKEND_URL: backendUrl,
      VITE_A_STOCK_TOKEN: process.env.A_STOCK_TOKEN || "",
    },
    logFile: files.frontendLog,
    pidFile: files.frontendPid,
  });
  await waitForHttp(frontendUrl, "frontend", frontendPid);

  log("restart complete");
  log(`backend:  ${backendUrl}`);
  log(`frontend: ${frontendUrl}`);
  log(`logs:     ${runtimeDir}`);

  launchBrowser({ url: frontendUrl });
}

export { restart, buildBackend, buildFrontend };

const isDirectExecution = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);

if (isDirectExecution) {
  try {
    if (process.argv.includes("--build-backend-only")) {
      buildBackend();
    } else {
      await restart();
    }
  } catch (error) {
    console.error(`[easy-stock] ${error.message}`);
    process.exitCode = 1;
  }
}
