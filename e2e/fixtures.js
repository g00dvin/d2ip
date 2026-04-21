import { test as base } from '@playwright/test';
import { execSync, spawn } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, '..');

// Shared container and pipeline state across tests in the same worker
let sharedContainer = null;
let sharedVolumeName = null;
let pipelineCompleted = false;

/**
 * Start the d2ip Docker container if not already running.
 */
async function getOrCreateContainer() {
  if (sharedContainer && !sharedContainer.killed) {
    console.log('[fixtures] Reusing existing container');
    return { container: sharedContainer, volumeName: sharedVolumeName };
  }

  // Clean up any existing containers on port 9099
  try {
    const existing = execSync('docker ps --filter "publish=9099" --format "{{.ID}}"', { stdio: 'pipe' }).toString().trim();
    if (existing) {
      console.log(`[fixtures] Killing existing container ${existing} on port 9099`);
      execSync(`docker kill ${existing}`, { stdio: 'pipe' });
      await new Promise(resolve => setTimeout(resolve, 2000));
    }
  } catch (e) {
    // No existing container
  }

  console.log('[fixtures] Creating new container...');
  const volumeName = `d2ip-e2e-${Date.now()}`;

  // Build image if needed
  try {
    console.log('[fixtures] Building docker image...');
    execSync('make docker', { cwd: ROOT, stdio: 'pipe' });
    console.log('[fixtures] Docker image built');
  } catch {
    console.log('[fixtures] Docker image already exists or build failed');
  }

  const container = spawn(
    'docker',
    [
      'run', '--rm', '-p', '9099:9099',
      '-v', `${volumeName}:/var/lib/d2ip`,
      '-e', 'D2IP_RESOLVER_UPSTREAM=1.1.1.1:53',
      '-e', 'D2IP_LOGGING_LEVEL=info',
      'd2ip:latest',
      'd2ip', 'serve',
    ],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );

  console.log(`[fixtures] Container spawned, waiting for HTTP server...`);
  // Wait for server to be ready (up to 60s)
  let output = '';
  container.stdout.on('data', (d) => { output += d.toString(); });
  container.stderr.on('data', (d) => { output += d.toString(); });

  const started = await new Promise((resolve) => {
    const check = setInterval(() => {
      if (output.includes('HTTP server listening')) {
        clearInterval(check);
        resolve(true);
      }
    }, 500);

    setTimeout(() => {
      clearInterval(check);
      resolve(false);
    }, 60_000);
  });

  if (!started) {
    container.kill();
    throw new Error(`d2ip container failed to start within 60s. Output:\n${output}`);
  }

  console.log('[fixtures] HTTP server is ready');
  sharedContainer = container;
  sharedVolumeName = volumeName;

  return { container, volumeName };
}

/**
 * Trigger the pipeline and wait for it to complete.
 * This fetches dlc.dat and populates the domain list provider.
 * Only runs once per test session.
 */
async function ensurePipelineCompleted() {
  if (pipelineCompleted) {
    console.log('[fixtures] Pipeline already completed, skipping');
    return;
  }

  console.log('[fixtures] Triggering pipeline...');
  // Trigger pipeline run
  const runResp = await fetch('http://127.0.0.1:9099/pipeline/run', { method: 'POST' });
  console.log(`[fixtures] Pipeline run response: ${runResp.status} ${runResp.statusText}`);
  if (!runResp.ok) {
    const err = await runResp.json().catch(() => ({}));
    throw new Error(`Pipeline trigger failed: ${err.error || runResp.statusText}`);
  }
  console.log('[fixtures] Pipeline triggered, waiting for completion...');

  // Poll status until complete (up to 900s - pipeline can take time to fetch dlc.dat)
  const started = await new Promise((resolve) => {
    const check = setInterval(async () => {
      try {
        const resp = await fetch('http://127.0.0.1:9099/pipeline/status');
        const data = await resp.json();
        console.log(`[fixtures] Pipeline status check: Running=${data.Running}, Report=${!!data.Report}`);
        // Go JSON uses capitalized field names (no JSON tags on RunStatus)
        // When pipeline completes, Running=false and Report is non-null
        if (!data.Running && data.Report) {
          clearInterval(check);
          resolve(true);
        }
      } catch (e) {
        console.log(`[fixtures] Pipeline status check error: ${e.message}`);
        // Ignore fetch errors during polling
      }
    }, 10000);

    setTimeout(() => {
      clearInterval(check);
      resolve(false);
    }, 900_000);
  });

  if (!started) {
    throw new Error('Pipeline failed to complete within 900s');
  }
  console.log('[fixtures] Pipeline completed successfully');
  pipelineCompleted = true;
}

export const test = base.extend({
  page: async ({ page }, use) => {
    console.log(`[fixtures] Starting test setup at ${new Date().toISOString()}`);
    const ctx = await getOrCreateContainer();
    console.log(`[fixtures] Container ready at ${new Date().toISOString()}`);
    try {
      // Ensure pipeline has completed (only runs once per session)
      await ensurePipelineCompleted();
      console.log(`[fixtures] Pipeline ready, starting test at ${new Date().toISOString()}`);
      await use(page);
    } finally {
      // Don't stop the container after each test - keep it for next test
    }
  },
});

// Cleanup hook to stop container when all tests are done
process.on('exit', () => {
  if (sharedContainer && !sharedContainer.killed) {
    sharedContainer.kill();
    if (sharedVolumeName) {
      try { execSync(`docker volume rm ${sharedVolumeName}`, { stdio: 'pipe' }); } catch (e) {}
    }
  }
});

export { expect } from '@playwright/test';
