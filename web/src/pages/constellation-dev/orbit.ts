export interface OrbitFlags {
  hover: boolean;
  drag: boolean;
  selected: boolean;
  hidden: boolean;
  busy: boolean;
  paused: boolean;
  reduced: boolean;
}

type OrbitFlagKey = keyof OrbitFlags;

// One monotonic clock. No timers, accumulated velocity, or input data.
export class OrbitController {
  flags: OrbitFlags;
  lastActivity: number;
  lastStep: number;
  phase: string;
  delay: number;
  ramp: number;
  speed: number;

  constructor(now = 0, reduced = false, { delay = 3000, ramp = 1000 }: { delay?: number; ramp?: number } = {}) {
    this.flags = { hover: false, drag: false, selected: false, hidden: false, busy: false, paused: false, reduced };
    this.lastActivity = now;
    this.lastStep = now;
    this.phase = reduced ? "reduced" : "waiting";
    this.delay = delay;
    this.ramp = ramp;
    this.speed = 0.055;
  }

  resumeNatural(now: number): void {
    if (this.blocker()) return;
    this.lastActivity = now - this.delay - this.ramp;
    this.lastStep = now;
    this.phase = "rotating";
  }

  activity(now: number): void {
    now = Math.max(now, this.lastActivity, this.lastStep);
    this.lastActivity = now;
    this.lastStep = now;
    this.phase = this.blocker() || "waiting";
  }

  configure(flags: Partial<OrbitFlags>, now: number): void {
    const entries = Object.entries(flags) as [OrbitFlagKey, boolean][];
    if (entries.some(([key, value]) => this.flags[key] !== value)) {
      Object.assign(this.flags, flags);
      this.activity(now);
    }
  }

  blocker(): OrbitFlagKey | undefined {
    const keys: OrbitFlagKey[] = ["reduced", "hidden", "paused", "selected", "drag", "hover", "busy"];
    return keys.find((key) => this.flags[key]);
  }

  canTick(): boolean {
    return !this.blocker();
  }

  integral(now: number): number {
    const t = (now - this.lastActivity - this.delay) / this.ramp;
    if (t <= 0) return 0;
    return (this.ramp / 1000) * (t < 1 ? t ** 3 - 0.5 * t ** 4 : t - 0.5);
  }

  step(now: number): number {
    now = Math.max(now, this.lastStep);
    const previous = Math.max(this.lastStep, now - 50);
    this.lastStep = now;
    const blocked = this.blocker();
    if (blocked) {
      this.phase = blocked;
      return 0;
    }
    const elapsed = now - this.lastActivity;
    this.phase = elapsed <= this.delay ? "waiting" : elapsed < this.delay + this.ramp ? "resuming" : "rotating";
    return this.speed * Math.max(0, this.integral(now) - this.integral(previous));
  }
}
