export interface Vec3 {
  x: number;
  y: number;
  z: number;
}

export interface CameraState {
  yaw: number;
  pitch: number;
  zoom: number;
  tx: number;
  ty: number;
  tz: number;
  panX: number;
  panY: number;
  cx?: number;
}

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface ProjectedPoint {
  x: number;
  y: number;
  depth: number;
  p: number;
  scale: number;
}

export interface ProjectedWork extends ProjectedPoint {
  w: PreparedWork;
}

export interface NeighborLink {
  key: string;
  weight: number;
}

export interface RawAuthor {
  id?: string;
  name?: string;
  avatarRef?: string;
  avatarURL?: string;
}

export interface RawWork {
  id: string;
  title?: string;
  publishedAt?: number;
  mediaKind?: string;
  coverRef?: string;
  coverURL?: string;
  cover?: string;
  author?: RawAuthor | string;
  authorAvatar?: string;
  [extra: string]: unknown;
}

export interface RawNeighbor {
  workId: string;
  to: string;
  score: number;
}

export interface RawScope {
  source: string;
  works: RawWork[];
  neighbors?: RawNeighbor[];
}

export interface ExplorationInput {
  snapshotId?: string;
  analysisVersion?: string | number;
  assets?: Record<string, string>;
  meta?: { coverage?: unknown };
  scopes: RawScope[];
}

export interface PreparedWork extends RawWork {
  key: string;
  source: string;
  seed: number;
  neighbors: NeighborLink[];
  pos: Vec3;
  community: string | null;
  color: string;
}

export interface PreparedPair {
  a: PreparedWork;
  b: PreparedWork;
  weight: number;
}

export interface BackgroundEdge {
  a: PreparedWork;
  b: PreparedWork;
  weight: number;
  rank: number;
}

export interface CommunitySummary {
  key: string;
  size: number;
  center: Vec3;
  radius: number;
}

export interface LayoutStats {
  works: number;
  linked: number;
  isolated: number;
  pairs: number;
  communities: number;
  components: number;
  elapsedMs: number;
  total?: number;
  edges?: number;
}

export interface PreparedModel {
  works: PreparedWork[];
  byKey: Map<string, PreparedWork>;
  communities: CommunitySummary[];
  stats: LayoutStats;
  layoutFingerprint: number;
  allKeys: Set<string>;
  initialStars: PreparedWork[];
  initialRelations?: BackgroundEdge[];
}

export type PlaceRectFn = (
  point: { x: number; y: number },
  width: number,
  height: number,
  occupied: Rect[],
  bounds: Rect,
  gap?: number,
) => Rect | null;
