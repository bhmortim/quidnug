/**
 * Quidnug Client SDK - Test Suite
 * 
 * Uses Node.js built-in test runner (available in Node 18+)
 */

import { describe, it, mock, beforeEach } from 'node:test';
import assert from 'node:assert';

// Mock browser APIs for Node.js environment
const mockCrypto = {
  subtle: {
    generateKey: mock.fn(async () => ({
      privateKey: { type: 'private' },
      publicKey: { type: 'public' }
    })),
    exportKey: mock.fn(async (format, key) => {
      if (format === 'pkcs8') return new ArrayBuffer(32);
      if (format === 'spki') return new ArrayBuffer(65);
      return new ArrayBuffer(0);
    }),
    importKey: mock.fn(async () => ({ type: 'private' })),
    sign: mock.fn(async () => new ArrayBuffer(64)),
    digest: mock.fn(async () => new ArrayBuffer(32))
  }
};

const mockFetch = mock.fn(async () => ({
  ok: true,
  json: async () => ({ status: 'ok' })
}));

const mockBtoa = (str) => Buffer.from(str, 'binary').toString('base64');
const mockAtob = (str) => Buffer.from(str, 'base64').toString('binary');

globalThis.window = {
  crypto: mockCrypto,
  btoa: mockBtoa,
  atob: mockAtob
};
globalThis.fetch = mockFetch;

const { default: QuidnugClient } = await import('./quidnug-client.js');

describe('QuidnugClient', () => {
  let client;

  beforeEach(() => {
    mock.reset();
    client = new QuidnugClient({ debug: false });
  });

  describe('constructor', () => {
    it('should create client with default options', () => {
      const c = new QuidnugClient();
      assert.strictEqual(c.debug, false);
      assert.deepStrictEqual(c.nodes, []);
    });

    it('should create client with custom options', () => {
      const c = new QuidnugClient({
        defaultNode: 'http://localhost:8080',
        debug: true
      });
      assert.strictEqual(c.debug, true);
      assert.strictEqual(c.defaultNode, 'http://localhost:8080');
    });
  });

  describe('addNode', () => {
    it('should add a node to the pool', () => {
      client.addNode('http://node1.example.com');
      assert.strictEqual(client.nodes.length, 1);
      assert.strictEqual(client.nodes[0].url, 'http://node1.example.com');
      assert.strictEqual(client.nodes[0].status, 'unknown');
    });

    it('should add multiple nodes', () => {
      client.addNode('http://node1.example.com');
      client.addNode('http://node2.example.com');
      assert.strictEqual(client.nodes.length, 2);
    });
  });

  describe('_getHealthyNode', () => {
    it('should throw error when no healthy nodes available', () => {
      assert.throws(
        () => client._getHealthyNode(),
        { message: 'No healthy nodes available' }
      );
    });

    it('should return a healthy node URL', () => {
      client.nodes = [
        { url: 'http://node1.example.com', status: 'healthy' }
      ];
      const url = client._getHealthyNode();
      assert.strictEqual(url, 'http://node1.example.com');
    });

    it('should only return healthy nodes', () => {
      client.nodes = [
        { url: 'http://unhealthy.example.com', status: 'unhealthy' },
        { url: 'http://healthy.example.com', status: 'healthy' }
      ];
      const url = client._getHealthyNode();
      assert.strictEqual(url, 'http://healthy.example.com');
    });
  });

  describe('_arrayBufferToBase64', () => {
    it('should convert ArrayBuffer to Base64', () => {
      const buffer = new Uint8Array([72, 101, 108, 108, 111]).buffer;
      const result = client._arrayBufferToBase64(buffer);
      assert.strictEqual(result, 'SGVsbG8=');
    });
  });

  describe('_base64ToArrayBuffer', () => {
    it('should convert Base64 to ArrayBuffer', () => {
      const result = client._base64ToArrayBuffer('SGVsbG8=');
      const bytes = new Uint8Array(result);
      assert.deepStrictEqual(Array.from(bytes), [72, 101, 108, 108, 111]);
    });
  });

  describe('importQuid', () => {
    it('should import quid with valid private and public keys', async () => {
      const result = await client.importQuid({
        privateKey: 'validPrivateKeyBase64',
        publicKey: 'validPublicKeyBase64'
      });
      assert.ok(result.id);
      assert.strictEqual(result.privateKey, 'validPrivateKeyBase64');
      assert.strictEqual(result.publicKey, 'validPublicKeyBase64');
      assert.strictEqual(result.imported, true);
    });

    it('should throw error when privateKey is missing', async () => {
      await assert.rejects(
        () => client.importQuid({ publicKey: 'someKey' }),
        { message: 'privateKey is required (Base64-encoded PKCS8 format)' }
      );
    });

    it('should throw error when publicKey is missing', async () => {
      await assert.rejects(
        () => client.importQuid({ privateKey: 'someKey' }),
        { message: 'publicKey is required (Base64-encoded SPKI format). Web Crypto API cannot derive public key from private key.' }
      );
    });

    it('should throw error when no keys provided', async () => {
      await assert.rejects(
        () => client.importQuid({}),
        { message: 'privateKey is required (Base64-encoded PKCS8 format)' }
      );
    });

    it('should throw error for null input', async () => {
      await assert.rejects(
        () => client.importQuid(null),
        /privateKey is required/
      );
    });

    it('should throw error for undefined input', async () => {
      await assert.rejects(
        () => client.importQuid(),
        /privateKey is required/
      );
    });
  });

  describe('createTrustTransaction', () => {
    it('should throw error without quid', async () => {
      await assert.rejects(
        () => client.createTrustTransaction({ trustee: 'b', domain: 'd', trustLevel: 0.5 }, null),
        { message: 'Valid quid with private key is required for signing' }
      );
    });

    it('should throw error for missing required parameters', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createTrustTransaction({ domain: 'd' }, quid),
        { message: 'Missing required parameters: trustee, domain, trustLevel' }
      );
    });

    it('should throw error for invalid trust level', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createTrustTransaction({ trustee: 'b', domain: 'd', trustLevel: 1.5 }, quid),
        { message: 'Trust level must be between 0.0 and 1.0' }
      );
    });

    it('should throw error for negative trust level', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createTrustTransaction({ trustee: 'b', domain: 'd', trustLevel: -0.1 }, quid),
        { message: 'Trust level must be between 0.0 and 1.0' }
      );
    });
  });

  describe('createIdentityTransaction', () => {
    it('should throw error without quid', async () => {
      await assert.rejects(
        () => client.createIdentityTransaction({ subjectQuid: 's', domain: 'd' }, null),
        { message: 'Valid quid with private key is required for signing' }
      );
    });

    it('should throw error for missing required parameters', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createIdentityTransaction({ domain: 'd' }, quid),
        { message: 'Missing required parameters: subjectQuid, domain' }
      );
    });
  });

  describe('createTitleTransaction', () => {
    it('should throw error without quid', async () => {
      await assert.rejects(
        () => client.createTitleTransaction({ assetQuid: 'a', domain: 'd', ownershipMap: [] }, null),
        { message: 'Valid quid with private key is required for signing' }
      );
    });

    it('should throw error for missing required parameters', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createTitleTransaction({ domain: 'd' }, quid),
        { message: 'Missing required parameters: assetQuid, domain, ownershipMap' }
      );
    });

    it('should throw error for empty ownershipMap', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createTitleTransaction({ assetQuid: 'a', domain: 'd', ownershipMap: [] }, quid),
        { message: 'OwnershipMap must be a non-empty array' }
      );
    });

    it('should throw error for ownership not summing to 100%', async () => {
      const quid = { id: 'test', privateKey: 'key' };
      await assert.rejects(
        () => client.createTitleTransaction({
          assetQuid: 'a',
          domain: 'd',
          ownershipMap: [{ ownerId: 'o1', percentage: 50 }]
        }, quid),
        /Total ownership percentage must equal 100%/
      );
    });
  });

  describe('submitTransaction', () => {
    it('should throw error for invalid transaction', async () => {
      await assert.rejects(
        () => client.submitTransaction(null),
        { message: 'Invalid transaction' }
      );
    });

    it('should throw error for transaction without type', async () => {
      await assert.rejects(
        () => client.submitTransaction({ txId: '123' }),
        { message: 'Invalid transaction' }
      );
    });

    it('should throw error for transaction without txId', async () => {
      await assert.rejects(
        () => client.submitTransaction({ type: 'TRUST' }),
        { message: 'Invalid transaction' }
      );
    });
  });

  describe('getTrustLevel', () => {
    it('should throw error for missing parameters', async () => {
      await assert.rejects(
        () => client.getTrustLevel(null, 'target', 'domain'),
        { message: 'Missing required parameters: observer, target, domain' }
      );
    });
  });

  describe('getIdentity', () => {
    it('should throw error for missing quidId', async () => {
      await assert.rejects(
        () => client.getIdentity(null),
        { message: 'Missing required parameter: quidId' }
      );
    });
  });

  describe('getAssetOwnership', () => {
    it('should throw error for missing assetId', async () => {
      await assert.rejects(
        () => client.getAssetOwnership(null),
        { message: 'Missing required parameter: assetId' }
      );
    });
  });

  describe('findTrustPath', () => {
    it('should throw error for missing parameters', async () => {
      await assert.rejects(
        () => client.findTrustPath(null, 'target', 'domain'),
        { message: 'Missing required parameters: sourceQuid, targetQuid, domain' }
      );
    });
  });

  describe('queryDomain', () => {
    it('should throw error for missing parameters', async () => {
      await assert.rejects(
        () => client.queryDomain(null, 'type', 'param'),
        { message: 'Missing required parameters: domain, type, param' }
      );
    });
  });

  describe('findNodesForDomain', () => {
    it('should throw error for missing domain', async () => {
      await assert.rejects(
        () => client.findNodesForDomain(null),
        { message: 'Missing required parameter: domain' }
      );
    });
  });

  describe('queryRelationalTrust', () => {
    it('should throw error for missing observer', async () => {
      await assert.rejects(
        () => client.queryRelationalTrust({ target: 'target123', domain: 'test' }),
        { message: 'Missing required parameters: observer, target' }
      );
    });

    it('should throw error for missing target', async () => {
      await assert.rejects(
        () => client.queryRelationalTrust({ observer: 'observer123', domain: 'test' }),
        { message: 'Missing required parameters: observer, target' }
      );
    });
  });

  describe('computeTransitiveTrust', () => {
    it('should return full trust for same observer and target', () => {
      const result = client.computeTransitiveTrust({}, 'quid1', 'quid1', 5);
      assert.strictEqual(result.trustLevel, 1.0);
      assert.deepStrictEqual(result.trustPath, ['quid1']);
    });

    it('should compute direct trust', () => {
      const graph = {
        'quid1': { 'quid2': 0.8 }
      };
      const result = client.computeTransitiveTrust(graph, 'quid1', 'quid2', 5);
      assert.strictEqual(result.trustLevel, 0.8);
      assert.deepStrictEqual(result.trustPath, ['quid1', 'quid2']);
    });

    it('should compute transitive trust with decay', () => {
      const graph = {
        'quid1': { 'quid2': 0.8 },
        'quid2': { 'quid3': 0.5 }
      };
      const result = client.computeTransitiveTrust(graph, 'quid1', 'quid3', 5);
      assert.strictEqual(result.trustLevel, 0.4);
      assert.deepStrictEqual(result.trustPath, ['quid1', 'quid2', 'quid3']);
    });

    it('should return zero trust when no path exists', () => {
      const graph = {
        'quid1': { 'quid2': 0.8 }
      };
      const result = client.computeTransitiveTrust(graph, 'quid1', 'quid3', 5);
      assert.strictEqual(result.trustLevel, 0);
      assert.deepStrictEqual(result.trustPath, []);
    });

    it('should handle cycles without infinite loop', () => {
      const graph = {
        'quid1': { 'quid2': 0.8 },
        'quid2': { 'quid1': 0.9, 'quid3': 0.7 }
      };
      const result = client.computeTransitiveTrust(graph, 'quid1', 'quid3', 5);
      const expected = 0.8 * 0.7;
      assert.strictEqual(result.trustLevel, expected);
    });

    it('should respect maxDepth limit', () => {
      const graph = {
        'quid1': { 'quid2': 0.9 },
        'quid2': { 'quid3': 0.9 },
        'quid3': { 'quid4': 0.9 }
      };
      const result = client.computeTransitiveTrust(graph, 'quid1', 'quid4', 2);
      assert.strictEqual(result.trustLevel, 0);
    });

    it('should find best path when multiple paths exist', () => {
      const graph = {
        'quid1': { 'quid2': 0.5, 'quid3': 0.9 },
        'quid2': { 'quid4': 0.5 },
        'quid3': { 'quid4': 0.9 }
      };
      const result = client.computeTransitiveTrust(graph, 'quid1', 'quid4', 5);
      assert.strictEqual(result.trustLevel, 0.81);
    });
  });
});

describe('createEventTransaction', () => {
  it('should throw error without quid', async () => {
    await assert.rejects(
      () => client.createEventTransaction({
        subjectId: 's',
        subjectType: 'QUID',
        eventType: 'e',
        domain: 'd',
        payload: {}
      }, null),
      { message: 'Valid quid with private key is required for signing' }
    );
  });

  it('should throw error for missing required parameters', async () => {
    const quid = { id: 'test', privateKey: 'key' };
    await assert.rejects(
      () => client.createEventTransaction({ domain: 'd' }, quid),
      { message: 'Missing required parameters: subjectId, subjectType, eventType, domain' }
    );
  });

  it('should throw error when neither payload nor payloadCID provided', async () => {
    const quid = { id: 'test', privateKey: 'key' };
    await assert.rejects(
      () => client.createEventTransaction({
        subjectId: 's',
        subjectType: 'QUID',
        eventType: 'e',
        domain: 'd'
      }, quid),
      { message: 'Either payload or payloadCID must be provided' }
    );
  });

  it('should throw error for invalid subjectType', async () => {
    const quid = { id: 'test', privateKey: 'key' };
    await assert.rejects(
      () => client.createEventTransaction({
        subjectId: 's',
        subjectType: 'INVALID',
        eventType: 'e',
        domain: 'd',
        payload: {}
      }, quid),
      { message: 'subjectType must be "QUID" or "TITLE"' }
    );
  });

  it('should accept QUID as subjectType', async () => {
    const quid = { id: 'test', privateKey: 'key' };
    await assert.rejects(
      () => client.createEventTransaction({
        subjectId: 's',
        subjectType: 'QUID',
        eventType: 'e',
        domain: 'd',
        payload: {}
      }, quid),
      /No healthy nodes available/
    );
  });

  it('should accept TITLE as subjectType', async () => {
    const quid = { id: 'test', privateKey: 'key' };
    await assert.rejects(
      () => client.createEventTransaction({
        subjectId: 's',
        subjectType: 'TITLE',
        eventType: 'e',
        domain: 'd',
        payload: {}
      }, quid),
      /No healthy nodes available/
    );
  });

  it('should accept payloadCID instead of payload', async () => {
    const quid = { id: 'test', privateKey: 'key' };
    await assert.rejects(
      () => client.createEventTransaction({
        subjectId: 's',
        subjectType: 'QUID',
        eventType: 'e',
        domain: 'd',
        payloadCID: 'QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG'
      }, quid),
      /No healthy nodes available/
    );
  });
});

describe('getEventStream', () => {
  it('should throw error for missing subjectId', async () => {
    await assert.rejects(
      () => client.getEventStream(null),
      { message: 'Missing required parameter: subjectId' }
    );
  });

  it('should throw error for undefined subjectId', async () => {
    await assert.rejects(
      () => client.getEventStream(undefined),
      { message: 'Missing required parameter: subjectId' }
    );
  });

  it('should throw error for empty string subjectId', async () => {
    await assert.rejects(
      () => client.getEventStream(''),
      { message: 'Missing required parameter: subjectId' }
    );
  });

  it('should require healthy node', async () => {
    await assert.rejects(
      () => client.getEventStream('subject123'),
      { message: 'No healthy nodes available' }
    );
  });

  it('should return null for NOT_FOUND error', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    mockFetch.mock.mockImplementation(async () => ({
      ok: true,
      json: async () => ({
        success: false,
        error: { code: 'NOT_FOUND', message: 'Event stream not found' }
      })
    }));

    const result = await client.getEventStream('nonexistent');
    assert.strictEqual(result, null);
  });

  it('should return stream metadata on success', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    const streamData = {
      subjectId: 'subject123',
      subjectType: 'QUID',
      latestSequence: 5,
      eventCount: 5
    };
    mockFetch.mock.mockImplementation(async () => ({
      ok: true,
      json: async () => ({ success: true, data: streamData })
    }));

    const result = await client.getEventStream('subject123');
    assert.deepStrictEqual(result, streamData);
  });
});

describe('getStreamEvents', () => {
  it('should throw error for missing subjectId', async () => {
    await assert.rejects(
      () => client.getStreamEvents(null),
      { message: 'Missing required parameter: subjectId' }
    );
  });

  it('should throw error for undefined subjectId', async () => {
    await assert.rejects(
      () => client.getStreamEvents(undefined),
      { message: 'Missing required parameter: subjectId' }
    );
  });

  it('should require healthy node', async () => {
    await assert.rejects(
      () => client.getStreamEvents('subject123'),
      { message: 'No healthy nodes available' }
    );
  });

  it('should return events and pagination on success', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    const eventsData = {
      data: [{ id: 'evt1', sequence: 1 }, { id: 'evt2', sequence: 2 }],
      pagination: { limit: 50, offset: 0, total: 2 }
    };
    mockFetch.mock.mockImplementation(async () => ({
      ok: true,
      json: async () => ({ success: true, data: eventsData })
    }));

    const result = await client.getStreamEvents('subject123');
    assert.deepStrictEqual(result.events, eventsData.data);
    assert.deepStrictEqual(result.pagination, eventsData.pagination);
  });

  it('should pass limit and offset options', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    mockFetch.mock.mockImplementation(async (url) => {
      assert.ok(url.includes('limit=10'));
      assert.ok(url.includes('offset=5'));
      return {
        ok: true,
        json: async () => ({ success: true, data: { data: [], pagination: {} } })
      };
    });

    await client.getStreamEvents('subject123', { limit: 10, offset: 5 });
  });
});

describe('pinToIPFS', () => {
  it('should throw error for empty content', async () => {
    await assert.rejects(
      () => client.pinToIPFS(null),
      { message: 'Content is required' }
    );
  });

  it('should throw error for undefined content', async () => {
    await assert.rejects(
      () => client.pinToIPFS(undefined),
      { message: 'Content is required' }
    );
  });

  it('should throw error for empty string', async () => {
    await assert.rejects(
      () => client.pinToIPFS(''),
      { message: 'Content is required' }
    );
  });

  it('should require healthy node', async () => {
    await assert.rejects(
      () => client.pinToIPFS('test content'),
      { message: 'No healthy nodes available' }
    );
  });

  it('should return CID on success with string content', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    mockFetch.mock.mockImplementation(async () => ({
      ok: true,
      json: async () => ({
        success: true,
        data: { cid: 'QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG' }
      })
    }));

    const cid = await client.pinToIPFS('test content');
    assert.strictEqual(cid, 'QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG');
  });

  it('should handle ArrayBuffer content', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    mockFetch.mock.mockImplementation(async (url, options) => {
      assert.strictEqual(options.headers['Content-Transfer-Encoding'], 'base64');
      return {
        ok: true,
        json: async () => ({
          success: true,
          data: { cid: 'QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG' }
        })
      };
    });

    const buffer = new ArrayBuffer(8);
    const cid = await client.pinToIPFS(buffer);
    assert.strictEqual(cid, 'QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG');
  });
});

describe('getFromIPFS', () => {
  it('should throw error for missing CID', async () => {
    await assert.rejects(
      () => client.getFromIPFS(null),
      { message: 'CID is required' }
    );
  });

  it('should throw error for undefined CID', async () => {
    await assert.rejects(
      () => client.getFromIPFS(undefined),
      { message: 'CID is required' }
    );
  });

  it('should throw error for empty string CID', async () => {
    await assert.rejects(
      () => client.getFromIPFS(''),
      { message: 'CID is required' }
    );
  });

  it('should require healthy node', async () => {
    await assert.rejects(
      () => client.getFromIPFS('QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG'),
      { message: 'No healthy nodes available' }
    );
  });

  it('should return ArrayBuffer on success', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    const testContent = new ArrayBuffer(8);
    mockFetch.mock.mockImplementation(async () => ({
      ok: true,
      arrayBuffer: async () => testContent
    }));

    const result = await client.getFromIPFS('QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG');
    assert.ok(result instanceof ArrayBuffer);
  });

  it('should throw error on failed retrieval', async () => {
    client.nodes = [{ url: 'http://node1.example.com', status: 'healthy' }];
    mockFetch.mock.mockImplementation(async () => ({
      ok: false,
      json: async () => ({
        success: false,
        error: { code: 'IPFS_UNAVAILABLE', message: 'IPFS service unavailable' }
      })
    }));

    await assert.rejects(
      () => client.getFromIPFS('QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG'),
      /IPFS service unavailable/
    );
  });
});

describe('_buildEventSignatureData', () => {
  it('should exclude signature and txId from data', () => {
    const tx = {
      type: 'EVENT',
      timestamp: 1000000,
      subjectId: 'subject123',
      signature: 'should-be-removed',
      txId: 'should-also-be-removed'
    };
    
    const data = client._buildEventSignatureData(tx);
    const parsed = JSON.parse(new TextDecoder().decode(data));
    
    assert.strictEqual(parsed.type, 'EVENT');
    assert.strictEqual(parsed.timestamp, 1000000);
    assert.strictEqual(parsed.subjectId, 'subject123');
    assert.strictEqual(parsed.signature, undefined);
    assert.strictEqual(parsed.txId, undefined);
  });

  it('should not modify original transaction', () => {
    const tx = {
      type: 'EVENT',
      signature: 'original-sig',
      txId: 'original-id'
    };
    
    client._buildEventSignatureData(tx);
    
    assert.strictEqual(tx.signature, 'original-sig');
    assert.strictEqual(tx.txId, 'original-id');
  });
});

describe('_parseResponse', () => {
  it('should return data on success', async () => {
    const mockResponse = {
      json: async () => ({ success: true, data: { test: 'value' } })
    };
    
    const result = await client._parseResponse(mockResponse);
    assert.deepStrictEqual(result, { test: 'value' });
  });

  it('should throw error with code on failure', async () => {
    const mockResponse = {
      json: async () => ({
        success: false,
        error: { code: 'TEST_ERROR', message: 'Test error message' }
      })
    };
    
    try {
      await client._parseResponse(mockResponse);
      assert.fail('Expected error to be thrown');
    } catch (error) {
      assert.strictEqual(error.code, 'TEST_ERROR');
      assert.strictEqual(error.message, 'Test error message');
    }
  });

  it('should use default error code when not provided', async () => {
    const mockResponse = {
      json: async () => ({ success: false, error: {} })
    };
    
    try {
      await client._parseResponse(mockResponse);
      assert.fail('Expected error to be thrown');
    } catch (error) {
      assert.strictEqual(error.code, 'UNKNOWN_ERROR');
    }
  });
});

console.log('QuidnugClient test suite loaded successfully');
