// Paddle.js overlay integration. Paddle Billing has no hosted redirect page:
// the browser loads Paddle.js and opens an in-page checkout for a transaction
// created server-side. See backend/payment/paddle for the transaction side.
import {
  initializePaddle,
  type Paddle,
  type PaddleEventData,
  type Environments,
} from '@paddle/paddle-js';

// Paddle.js fires events through a single global callback set at init time, so
// we route them through a swappable module-level handler that the active
// checkout registers before opening the overlay.
let paddlePromise: Promise<Paddle | undefined> | null = null;
let eventHandler: ((e: PaddleEventData) => void) | null = null;

/**
 * Registers the callback that receives Paddle.js checkout events, or clears it
 * with null. Only one handler is active at a time.
 */
export function setPaddleEventHandler(fn: ((e: PaddleEventData) => void) | null): void {
  eventHandler = fn;
}

/**
 * Lazily initialises Paddle.js once per page load and returns the instance.
 *
 * @param token       Paddle client-side token (browser-safe), from site config.
 * @param environment "sandbox" or "production"; anything else is treated as production.
 * @returns The Paddle instance, or undefined if the SDK failed to load.
 */
export function getPaddle(token: string, environment: string): Promise<Paddle | undefined> {
  if (paddlePromise) return paddlePromise;
  const env: Environments = environment === 'sandbox' ? 'sandbox' : 'production';
  const p = initializePaddle({
    token,
    environment: env,
    eventCallback: (e: PaddleEventData) => eventHandler?.(e),
  });
  paddlePromise = p;
  return p;
}
