export interface PoolRecord {
  ready?: boolean;
  failed?: boolean;
}

// Bounded decoded images, not a lifetime request quota. Active/fading covers stay pinned.
export class ImagePool<T extends PoolRecord = PoolRecord> {
  limit: number;
  map: Map<string, T>;
  pins: Set<string>;
  evictions: number;
  onEvict: (value: T) => void;

  constructor(limit = 128, onEvict: (value: T) => void = () => {}) {
    this.limit = limit;
    this.map = new Map();
    this.pins = new Set();
    this.evictions = 0;
    this.onEvict = onEvict;
  }

  get size(): number {
    return this.map.size;
  }

  has(key: string): boolean {
    return this.map.has(key);
  }

  get(key: string): T | undefined {
    const v = this.map.get(key);
    if (v) {
      this.map.delete(key);
      this.map.set(key, v);
    }
    return v;
  }

  values(): MapIterator<T> {
    return this.map.values();
  }

  retain(keys: Iterable<string | null | undefined>): void {
    const next = new Set<string>();
    for (const key of keys) {
      if (key) next.add(key);
    }
    this.pins = next;
  }

  set(key: string, value: T): boolean {
    if (this.map.has(key)) {
      this.map.delete(key);
      this.map.set(key, value);
      return true;
    }
    if (this.map.size >= this.limit) {
      const victim = [...this.map].find(([k, v]) => !this.pins.has(k) && (v.ready || v.failed));
      if (!victim) return false;
      this.map.delete(victim[0]);
      this.onEvict(victim[1]);
      this.evictions++;
    }
    this.map.set(key, value);
    return true;
  }

  clear(): void {
    for (const value of this.map.values()) this.onEvict(value);
    this.map.clear();
    this.pins.clear();
  }
}
