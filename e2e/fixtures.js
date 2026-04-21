import { test as base } from '@playwright/test';
import { execSync, spawn } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');

/**
 * Start the d2ip Docker container and return its lifecycle helpers.
 * Uses a fresh data volume per test run to ensure clean state.
 */
async function startContainer() {
  const volumeName = `d2ip-e2e-${Date.now()}`;

  // Build image if needed
  try {
    execSync('make docker', { cwd: ROOT, stdio: 'pipe' });
  } catch {
    // Image might already exist
  }

  const container = spawn(
    'docker',
    [
      'run', '--rm', '-p', '9099:9099',
      '-v', `${volumeName}:/var/lib/d2ip`,
      '-e', 'D2IP_RESOLVER_UPSTREAM=1.1.1.1:53',
      '-e', 'D2IP_LOGGING_LEVEL=info',
      'ghcr.io/g00dvin/d2ip:latest',
      'd2ip', 'serve',
    ],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );

  // Wait for server to be ready (up to 30s)
  const started = await new Promise((resolve) => {
    let output = '';
    container.stdout.on('data', (d) => { output += d.toString(); });
    container.stderr.on('data', (d) => { output += d.toString(); });

    const check = setInterval(() => {
      if (output.includes('HTTP server listening')) {
        clearInterval(check);
        resolve(true);
      }
    }, 500);

    setTimeout(() => {
      clearInterval(check);
      resolve(false);
    }, 30_000);
  });

  if (!started) {
    container.kill();
    throw new Error('d2ip container failed to start within 30s');
  }

  return {
    container,
    stop: () => {
      container.kill();
      // Clean up volume
      try { execSync(`docker volume rm ${volumeName}`, { stdio: 'pipe' }); } catch {}
    },
  };
}

export const test = base.extend({
  page: async ({ page }, use) => {
    const ctx = await startContainer();
    try {
      await use(page);
    } finally {
      ctx.stop();
    }
  },
});

export { expect } from '@playwright/test';
