export interface RetryOptions {
  retryMs: number;
  signal?: AbortSignal;
  shouldRetry?: () => boolean;
  onFailure?: (error: unknown, attempt: number) => void;
}

function aborted() {
  const error = new Error('startup readiness wait aborted');
  error.name = 'AbortError';
  return error;
}

function wait(milliseconds: number, signal?: AbortSignal) {
  if (signal?.aborted) return Promise.reject(aborted());
  return new Promise<void>((resolve, reject) => {
    const timer = setTimeout(done, milliseconds);
    function done() {
      signal?.removeEventListener('abort', cancel);
      resolve();
    }
    function cancel() {
      clearTimeout(timer);
      signal?.removeEventListener('abort', cancel);
      reject(aborted());
    }
    signal?.addEventListener('abort', cancel, { once: true });
  });
}

/** Retry a startup dependency until it is ready or the process is shutting down. */
export async function retryUntilReady(task: () => Promise<void>, options: RetryOptions) {
  let attempt = 0;
  while (!options.signal?.aborted && (options.shouldRetry?.() ?? true)) {
    attempt += 1;
    try {
      await task();
      return;
    } catch (error) {
      options.onFailure?.(error, attempt);
      if (!(options.shouldRetry?.() ?? true)) throw aborted();
      await wait(options.retryMs, options.signal);
    }
  }
  throw aborted();
}

export function isAbortError(error: unknown) {
  return error instanceof Error && error.name === 'AbortError';
}
