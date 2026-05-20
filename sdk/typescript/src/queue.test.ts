import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { EventQueue } from './queue';

describe('EventQueue', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('enqueue increases size', () => {
    const q = new EventQueue<string>(() => Promise.resolve(), { flushIntervalMs: 0 });
    q.enqueue('a');
    q.enqueue('b');
    expect(q.size).toBe(2);
  });

  it('flush drains up to batchSize items and calls onFlush', async () => {
    const flushed: string[][] = [];
    const q = new EventQueue<string>(
      (items) => { flushed.push([...items]); return Promise.resolve(); },
      // batchSize > item count so no auto-flush fires; only explicit flush is called
      { batchSize: 10, flushIntervalMs: 0 },
    );
    q.enqueue('a');
    q.enqueue('b');
    q.enqueue('c');
    await q.flush();
    expect(flushed).toEqual([['a', 'b', 'c']]);
    expect(q.size).toBe(0);
  });

  it('auto-flush fires synchronously when batchSize threshold is reached', async () => {
    const flushed: string[][] = [];
    const q = new EventQueue<string>(
      (items) => { flushed.push([...items]); return Promise.resolve(); },
      { batchSize: 3, flushIntervalMs: 0 },
    );
    q.enqueue('a');
    q.enqueue('b');
    // Third item reaches batchSize — auto-flush splices items before the first await
    q.enqueue('c');
    q.enqueue('d'); // added to now-empty queue
    await q.flush(); // flushes 'd'
    expect(flushed).toEqual([['a', 'b', 'c'], ['d']]);
    expect(q.size).toBe(0);
  });

  it('flushAll drains all items across multiple batches', async () => {
    const flushed: string[][] = [];
    const q = new EventQueue<string>(
      (items) => { flushed.push([...items]); return Promise.resolve(); },
      { batchSize: 2, flushIntervalMs: 0 },
    );
    q.enqueue('a');
    q.enqueue('b');
    q.enqueue('c');
    await q.flushAll();
    expect(flushed).toEqual([['a', 'b'], ['c']]);
    expect(q.size).toBe(0);
  });

  it('flush is a no-op when queue is empty', async () => {
    let calls = 0;
    const q = new EventQueue<string>(() => { calls++; return Promise.resolve(); }, { flushIntervalMs: 0 });
    await q.flush();
    expect(calls).toBe(0);
  });

  it('overflow drop_oldest drops the oldest item when full', () => {
    const q = new EventQueue<number>(
      () => Promise.resolve(),
      { maxSize: 3, batchSize: 100, flushIntervalMs: 0, overflowPolicy: 'drop_oldest' },
    );
    q.enqueue(1);
    q.enqueue(2);
    q.enqueue(3);
    q.enqueue(4); // should drop 1
    expect(q.size).toBe(3);

    const captured: number[][] = [];
    const q2 = new EventQueue<number>(
      (items) => { captured.push([...items]); return Promise.resolve(); },
      { maxSize: 3, batchSize: 100, flushIntervalMs: 0, overflowPolicy: 'drop_oldest' },
    );
    q2.enqueue(1);
    q2.enqueue(2);
    q2.enqueue(3);
    q2.enqueue(4);
    void q2.flushAll().then(() => {
      expect(captured[0]).toEqual([2, 3, 4]);
    });
  });

  it('overflow drop_newest silently drops the incoming item when full', () => {
    const q = new EventQueue<number>(
      () => Promise.resolve(),
      { maxSize: 2, batchSize: 100, flushIntervalMs: 0, overflowPolicy: 'drop_newest' },
    );
    q.enqueue(1);
    q.enqueue(2);
    q.enqueue(3); // dropped
    expect(q.size).toBe(2);
  });

  it('timer triggers flushAll after interval elapses', async () => {
    const flushed: string[][] = [];
    const q = new EventQueue<string>(
      (items) => { flushed.push([...items]); return Promise.resolve(); },
      { flushIntervalMs: 1000, batchSize: 100 },
    );
    q.enqueue('x');
    expect(flushed).toHaveLength(0);
    await vi.advanceTimersByTimeAsync(1000);
    expect(flushed).toEqual([['x']]);
    await q.shutdown();
  });

  it('shutdown flushes remaining items and stops the timer', async () => {
    const flushed: string[][] = [];
    const q = new EventQueue<string>(
      (items) => { flushed.push([...items]); return Promise.resolve(); },
      { flushIntervalMs: 5000, batchSize: 100 },
    );
    q.enqueue('a');
    q.enqueue('b');
    await q.shutdown();
    expect(flushed).toEqual([['a', 'b']]);
    expect(q.size).toBe(0);
  });
});
