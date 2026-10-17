import type { Health } from './types';

export type Freshness = 'live' | 'stale' | 'unavailable';

/** Derives freshness from observed_at + stale_after_seconds. Missing health data is unavailable. */
export function freshnessOf(health: Health | undefined, now: Date | number = Date.now()): Freshness {
  if (!health) return 'unavailable';
  const observed = Date.parse(health.observed_at);
  if (Number.isNaN(observed)) return 'unavailable';
  const ttl = health.stale_after_seconds;
  if (!ttl) return 'live';
  const nowMs = typeof now === 'number' ? now : now.getTime();
  return nowMs - observed > ttl * 1000 ? 'stale' : 'live';
}
