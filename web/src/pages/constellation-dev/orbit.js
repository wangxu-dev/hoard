// One monotonic clock. No timers, accumulated velocity, focus latch or input data.
export class OrbitController {
  constructor(now = 0, reduced = false) {
    this.flags = { hover: false, drag: false, selected: false, hidden: false, busy: false, paused: false, reduced };
    this.lastActivity = now;
    this.lastStep = now;
    this.phase = reduced ? 'reduced' : 'waiting';
    this.delay = 3000;
    this.ramp = 1000;
    this.speed = .055;
  }
  activity(now) { now = Math.max(now, this.lastActivity, this.lastStep); this.lastActivity = now; this.lastStep = now; this.phase = this.blocker() || 'waiting'; }
  configure(flags, now) {
    if (Object.entries(flags).some(([key, value]) => this.flags[key] !== value)) {
      Object.assign(this.flags, flags);
      this.activity(now);
    }
  }
  blocker() { return ['reduced', 'hidden', 'paused', 'selected', 'drag', 'hover', 'busy'].find(key => this.flags[key]); }
  canTick() { return !this.blocker(); }
  integral(now) {
    const t = (now - this.lastActivity - this.delay) / this.ramp;
    if (t <= 0) return 0;
    return this.ramp / 1000 * (t < 1 ? t ** 3 - .5 * t ** 4 : t - .5);
  }
  step(now) {
    now = Math.max(now, this.lastStep);
    const previous = Math.max(this.lastStep, now - 50);
    this.lastStep = now;
    const blocked = this.blocker();
    if (blocked) { this.phase = blocked; return 0; }
    const elapsed = now - this.lastActivity;
    this.phase = elapsed <= this.delay ? 'waiting' : elapsed < this.delay + this.ramp ? 'resuming' : 'rotating';
    return this.speed * Math.max(0, this.integral(now) - this.integral(previous));
  }
}
