// Pages already-resident native snapshot rows. No bridge request or image prefetch.
export class SnapshotPages<T extends { key: string }> {
  readonly size: number;
  items: T[] = [];
  count = 0;
  loading = false;
  generation = 0;
  constructor(items: readonly T[] = [], size=24) { this.size=size; this.reset(items); }
  reset(items: readonly T[]) { this.generation++; this.loading=false; this.items=[...new Map(items.map(item=>[item.key,item])).values()]; this.count=Math.min(this.size,this.items.length); }
  get more() { return this.count<this.items.length; }
  get visible() { return this.items.slice(0,this.count); }
  async next() {
    if(this.loading||!this.more)return false;
    const generation=this.generation;this.loading=true;
    await Promise.resolve(); // A cancellable UI turn, not a pretend network request.
    if(generation!==this.generation)return false;
    this.count=Math.min(this.count+this.size,this.items.length);this.loading=false;return true;
  }
  cancel() { this.generation++; this.loading=false; }
}
export const PAGE_THRESHOLD=320;
export function nearPageEnd(scrollTop:number,viewport:number,total:number) { return viewport>0&&total-scrollTop-viewport<=PAGE_THRESHOLD; }
