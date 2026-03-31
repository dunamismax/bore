export type RateLimitResult = {
  allowed: boolean;
  limit: number;
  remaining: number;
  resetAt: number;
};

type Bucket = {
  count: number;
  resetAt: number;
};

export class InMemoryRateLimiter {
  private readonly buckets = new Map<string, Bucket>();
  private readonly maxRequests: number;
  private readonly windowMs: number;

  constructor({
    maxRequests,
    windowMs,
  }: {
    maxRequests: number;
    windowMs: number;
  }) {
    this.maxRequests = maxRequests;
    this.windowMs = windowMs;
  }

  check(key: string, now = Date.now()): RateLimitResult {
    const current = this.buckets.get(key);

    if (!current || current.resetAt <= now) {
      const next: Bucket = {
        count: 1,
        resetAt: now + this.windowMs,
      };

      this.buckets.set(key, next);

      return {
        allowed: true,
        limit: this.maxRequests,
        remaining: Math.max(0, this.maxRequests - next.count),
        resetAt: next.resetAt,
      };
    }

    if (current.count >= this.maxRequests) {
      return {
        allowed: false,
        limit: this.maxRequests,
        remaining: 0,
        resetAt: current.resetAt,
      };
    }

    current.count += 1;
    this.buckets.set(key, current);

    return {
      allowed: true,
      limit: this.maxRequests,
      remaining: Math.max(0, this.maxRequests - current.count),
      resetAt: current.resetAt,
    };
  }
}
