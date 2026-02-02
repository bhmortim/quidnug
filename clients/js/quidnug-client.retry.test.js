/**
 * Retry behavior tests for QuidnugClient
 * 
 * These tests verify the exponential backoff retry logic for HTTP requests.
 */

import { describe, it, mock, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert';

// Mock globals before importing the client
const mockCrypto = {
  subtle: {
    generateKey: mock.fn(async () => ({
      privateKey: { type: 'private' },
      publicKey: { type: 'public' }
    })),
    exportKey: mock.fn(async (format) => {
      if (format === 'pkcs8') return new ArrayBuffer(32);
      if (format === 'spki') return new ArrayBuffer(65);
      return new ArrayBuffer(0);
    }),
    importKey: mock.fn(async () => ({ type: 'private' })),
    sign: mock.fn(async () => new ArrayBuffer(64)),
    digest: mock.fn(async () => new ArrayBuffer(32))
  }
};

const originalFetch = globalThis.fetch;
let mockFetch;

globalThis.window = {
  crypto: mockCrypto,
  btoa: (str) => Buffer.from(str, 'binary').toString('base64'),
  atob: (str) => Buffer.from(str, 'base64').toString('binary')
};

globalThis.fetch = (...args) => mockFetch(...args);

// Import after mocks are set up
const QuidnugClient = (await import('./quidnug-client.js')).default;

describe('QuidnugClient Retry Logic', () => {
  let client;
  let fetchCallCount;
  let fetchCallTimes;

  beforeEach(() => {
    fetchCallCount = 0;
    fetchCallTimes = [];
    
    // Create client with short delays for testing
    client = new QuidnugClient({
      defaultNode: 'http://localhost:8080',
      maxRetries: 3,
      retryBaseDelayMs: 10 // Short delay for tests
    });
    
    // Mark node as healthy
    client.nodes[0].status = 'healthy';
  });

  afterEach(() => {
    mock.reset();
  });

  describe('_fetchWithRetry', () => {
    it('should succeed on first attempt when server responds OK', async () => {
      mockFetch = mock.fn(async () => ({
        ok: true,
        status: 200,
        json: async () => ({ data: 'success' })
      }));

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.ok, true);
      assert.strictEqual(mockFetch.mock.calls.length, 1);
    });

    it('should retry on network error and succeed', async () => {
      let callCount = 0;
      mockFetch = mock.fn(async () => {
        callCount++;
        if (callCount < 3) {
          throw new Error('Network error');
        }
        return {
          ok: true,
          status: 200,
          json: async () => ({ data: 'success' })
        };
      });

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.ok, true);
      assert.strictEqual(mockFetch.mock.calls.length, 3);
    });

    it('should retry on 5xx server errors and succeed', async () => {
      let callCount = 0;
      mockFetch = mock.fn(async () => {
        callCount++;
        if (callCount < 3) {
          return {
            ok: false,
            status: 503,
            json: async () => ({ error: 'Service unavailable' })
          };
        }
        return {
          ok: true,
          status: 200,
          json: async () => ({ data: 'success' })
        };
      });

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.ok, true);
      assert.strictEqual(mockFetch.mock.calls.length, 3);
    });

    it('should retry on 429 rate limit and succeed', async () => {
      let callCount = 0;
      mockFetch = mock.fn(async () => {
        callCount++;
        if (callCount < 2) {
          return {
            ok: false,
            status: 429,
            json: async () => ({ error: 'Rate limited' })
          };
        }
        return {
          ok: true,
          status: 200,
          json: async () => ({ data: 'success' })
        };
      });

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.ok, true);
      assert.strictEqual(mockFetch.mock.calls.length, 2);
    });

    it('should NOT retry on 4xx client errors (except 429)', async () => {
      mockFetch = mock.fn(async () => ({
        ok: false,
        status: 400,
        json: async () => ({ error: 'Bad request' })
      }));

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.status, 400);
      assert.strictEqual(mockFetch.mock.calls.length, 1);
    });

    it('should NOT retry on 404 not found', async () => {
      mockFetch = mock.fn(async () => ({
        ok: false,
        status: 404,
        json: async () => ({ error: 'Not found' })
      }));

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.status, 404);
      assert.strictEqual(mockFetch.mock.calls.length, 1);
    });

    it('should NOT retry on 401 unauthorized', async () => {
      mockFetch = mock.fn(async () => ({
        ok: false,
        status: 401,
        json: async () => ({ error: 'Unauthorized' })
      }));

      const response = await client._fetchWithRetry('http://localhost:8080/api/test');
      
      assert.strictEqual(response.status, 401);
      assert.strictEqual(mockFetch.mock.calls.length, 1);
    });

    it('should throw after exhausting all retries', async () => {
      mockFetch = mock.fn(async () => {
        throw new Error('Persistent network error');
      });

      await assert.rejects(
        async () => client._fetchWithRetry('http://localhost:8080/api/test'),
        { message: 'Persistent network error' }
      );
      
      // Initial attempt + 3 retries = 4 total calls
      assert.strictEqual(mockFetch.mock.calls.length, 4);
    });

    it('should use exponential backoff delays', async () => {
      const callTimes = [];
      mockFetch = mock.fn(async () => {
        callTimes.push(Date.now());
        if (callTimes.length < 4) {
          throw new Error('Network error');
        }
        return { ok: true, status: 200 };
      });

      // Use a client with measurable delays
      const testClient = new QuidnugClient({
        maxRetries: 3,
        retryBaseDelayMs: 50
      });

      await testClient._fetchWithRetry('http://localhost:8080/api/test');

      // Verify delays increase exponentially
      // Expected delays: ~50ms, ~100ms, ~200ms (plus jitter)
      const delay1 = callTimes[1] - callTimes[0];
      const delay2 = callTimes[2] - callTimes[1];
      const delay3 = callTimes[3] - callTimes[2];

      // Allow for some timing variance but verify exponential growth
      assert.ok(delay1 >= 40, `First delay ${delay1}ms should be >= 40ms`);
      assert.ok(delay2 >= 80, `Second delay ${delay2}ms should be >= 80ms`);
      assert.ok(delay3 >= 160, `Third delay ${delay3}ms should be >= 160ms`);
      assert.ok(delay2 > delay1, 'Second delay should be greater than first');
      assert.ok(delay3 > delay2, 'Third delay should be greater than second');
    });

    it('should respect custom maxRetries parameter', async () => {
      mockFetch = mock.fn(async () => {
        throw new Error('Network error');
      });

      await assert.rejects(
        async () => client._fetchWithRetry('http://localhost:8080/api/test', {}, 1),
        { message: 'Network error' }
      );
      
      // Initial attempt + 1 retry = 2 total calls
      assert.strictEqual(mockFetch.mock.calls.length, 2);
    });

    it('should respect custom baseDelayMs parameter', async () => {
      const callTimes = [];
      mockFetch = mock.fn(async () => {
        callTimes.push(Date.now());
        if (callTimes.length < 2) {
          throw new Error('Network error');
        }
        return { ok: true, status: 200 };
      });

      await client._fetchWithRetry('http://localhost:8080/api/test', {}, 1, 100);

      const delay = callTimes[1] - callTimes[0];
      assert.ok(delay >= 90, `Delay ${delay}ms should be >= 90ms with 100ms base`);
    });
  });

  describe('Integration with API methods', () => {
    it('getTrustLevel should use retry logic', async () => {
      let callCount = 0;
      mockFetch = mock.fn(async () => {
        callCount++;
        if (callCount < 2) {
          throw new Error('Network error');
        }
        return {
          ok: true,
          status: 200,
          json: async () => ({
            observer: 'abc123',
            target: 'def456',
            trustLevel: 0.8,
            trustPath: ['abc123', 'def456'],
            pathDepth: 1,
            domain: 'test.com'
          })
        };
      });

      const result = await client.getTrustLevel('abc123', 'def456', 'test.com');
      
      assert.strictEqual(result.trustLevel, 0.8);
      assert.strictEqual(mockFetch.mock.calls.length, 2);
    });

    it('getBlocks should use retry logic', async () => {
      let callCount = 0;
      mockFetch = mock.fn(async () => {
        callCount++;
        if (callCount < 2) {
          return { ok: false, status: 500 };
        }
        return {
          ok: true,
          status: 200,
          json: async () => ({
            data: [],
            pagination: { total: 0, limit: 50, offset: 0, hasMore: false }
          })
        };
      });

      const result = await client.getBlocks();
      
      assert.deepStrictEqual(result.data, []);
      assert.strictEqual(mockFetch.mock.calls.length, 2);
    });

    it('submitTransaction should use retry logic', async () => {
      let callCount = 0;
      mockFetch = mock.fn(async () => {
        callCount++;
        if (callCount < 2) {
          return { ok: false, status: 503 };
        }
        return {
          ok: true,
          status: 200,
          json: async () => ({ status: 'accepted', txId: 'abc123' })
        };
      });

      const result = await client.submitTransaction({
        type: 'TRUST',
        txId: 'test123',
        trustee: 'def456',
        trustLevel: 0.5
      });
      
      assert.strictEqual(result.status, 'accepted');
      assert.strictEqual(mockFetch.mock.calls.length, 2);
    });
  });

  describe('Constructor configuration', () => {
    it('should use default retry values when not specified', () => {
      const defaultClient = new QuidnugClient();
      
      assert.strictEqual(defaultClient.maxRetries, 3);
      assert.strictEqual(defaultClient.retryBaseDelayMs, 1000);
    });

    it('should accept custom retry configuration', () => {
      const customClient = new QuidnugClient({
        maxRetries: 5,
        retryBaseDelayMs: 500
      });
      
      assert.strictEqual(customClient.maxRetries, 5);
      assert.strictEqual(customClient.retryBaseDelayMs, 500);
    });

    it('should accept zero maxRetries to disable retries', () => {
      const noRetryClient = new QuidnugClient({ maxRetries: 0 });
      
      assert.strictEqual(noRetryClient.maxRetries, 0);
    });
  });
});
