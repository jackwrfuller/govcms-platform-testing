const { execSync } = require("child_process");
const path = require("path");

const APP_URL = process.env.APP_URL || "http://app:3000";
const POLL_INTERVAL = parseInt(process.env.POLL_INTERVAL || "5000", 10);
const CYPRESS_CONFIG = process.env.CYPRESS_CONFIG || "/app/cypress/cypress.config.js";
const WORKER_ID = process.env.WORKER_ID || `worker-${process.pid}`;

console.log(`[${WORKER_ID}] Starting Cypress worker`);
console.log(`[${WORKER_ID}] App URL: ${APP_URL}`);
console.log(`[${WORKER_ID}] Poll interval: ${POLL_INTERVAL}ms`);

async function poll() {
  try {
    const res = await fetch(`${APP_URL}/api/worker/poll`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ worker_id: WORKER_ID }),
    });

    if (res.status === 204) {
      return null;
    }

    if (!res.ok) {
      console.error(`[${WORKER_ID}] Poll error: ${res.status}`);
      return null;
    }

    return await res.json();
  } catch (err) {
    console.error(`[${WORKER_ID}] Poll failed: ${err.message}`);
    return null;
  }
}

async function submitResult(result) {
  try {
    const res = await fetch(`${APP_URL}/api/worker/result`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(result),
    });

    if (!res.ok) {
      console.error(`[${WORKER_ID}] Submit error: ${res.status}`);
    }
  } catch (err) {
    console.error(`[${WORKER_ID}] Submit failed: ${err.message}`);
  }
}

function runCypress(job) {
  const startTime = Date.now();
  let stdout = "";
  let stderr = "";
  let exitCode = 0;

  try {
    stdout = execSync(
      `npx cypress run --config-file "${CYPRESS_CONFIG}" --config baseUrl="${job.canary_url}"`,
      {
        cwd: "/app",
        timeout: 600000, // 10 minutes
        encoding: "utf-8",
        env: {
          ...process.env,
          CYPRESS_SITE_NAME: job.site_name,
        },
        stdio: ["pipe", "pipe", "pipe"],
      }
    );
  } catch (err) {
    exitCode = err.status || 1;
    stdout = err.stdout || "";
    stderr = err.stderr || "";
  }

  const durationMs = Date.now() - startTime;

  // Parse test counts from Cypress output
  const stats = parseCypressOutput(stdout);

  return {
    result_id: job.result_id,
    status: exitCode === 0 ? "passed" : "failed",
    exit_code: exitCode,
    tests_total: stats.total,
    tests_passed: stats.passed,
    tests_failed: stats.failed,
    duration_ms: durationMs,
    stdout: truncate(stripAnsi(stdout), 50000),
    stderr: truncate(stripAnsi(stderr), 50000),
    error_msg: exitCode !== 0 ? `Exit code: ${exitCode}` : "",
  };
}

function parseCypressOutput(output) {
  const stats = { total: 0, passed: 0, failed: 0 };
  if (!output) return stats;

  // Match Cypress summary line patterns like:
  //   Tests:  3
  //   Passing: 2
  //   Failing: 1
  // Or the table format:
  //   ✔ 2 of 3 passed
  const passingMatch = output.match(/(\d+)\s+passing/i);
  const failingMatch = output.match(/(\d+)\s+failing/i);
  const pendingMatch = output.match(/(\d+)\s+pending/i);

  if (passingMatch) stats.passed = parseInt(passingMatch[1], 10);
  if (failingMatch) stats.failed = parseInt(failingMatch[1], 10);

  stats.total = stats.passed + stats.failed;
  if (pendingMatch) stats.total += parseInt(pendingMatch[1], 10);

  return stats;
}

function stripAnsi(str) {
  if (!str) return "";
  // eslint-disable-next-line no-control-regex
  return str.replace(/\x1B\[[0-9;]*[A-Za-z]/g, "");
}

function truncate(str, maxLen) {
  if (!str || str.length <= maxLen) return str || "";
  return str.substring(str.length - maxLen);
}

async function loop() {
  while (true) {
    const job = await poll();

    if (job) {
      console.log(
        `[${WORKER_ID}] Running tests for ${job.site_name} (${job.canary_url})`
      );
      const result = runCypress(job);
      console.log(
        `[${WORKER_ID}] ${job.site_name}: ${result.status} (${result.tests_passed}/${result.tests_total} passed, ${result.duration_ms}ms)`
      );
      await submitResult(result);
    } else {
      await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL));
    }
  }
}

loop().catch((err) => {
  console.error(`[${WORKER_ID}] Fatal error: ${err.message}`);
  process.exit(1);
});
