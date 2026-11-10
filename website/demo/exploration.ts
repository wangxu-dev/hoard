import catalog from "./exploration-catalog.json";

// Dates, author labels and identifiers belong to the demo, not the source library.
export function createExplorationSnapshot(seed: number, english = false) {
  let state = seed >>> 0;
  const random = () => { state = (Math.imul(state, 1664525) + 1013904223) >>> 0; return state / 4294967296; };
  const shuffled = catalog.items.map(item => ({ item, rank: random() })).sort((a, b) => a.rank - b.rank).map(({ item }) => item);
  const ids = new Map(shuffled.map((item, index) => [item.id, `demo-${seed.toString(16)}-${index}`]));
  const scopes = ["douyin", "bilibili"].map(source => {
    const works = shuffled.filter(item => item.source === source).map((item, index) => {
      const year = 2020 + Math.floor(random() * 6), month = Math.floor(random() * 12);
      return {
        id: ids.get(item.id)!, source, title: item.title, mediaKind: item.mediaKind,
        publishedAt: Date.UTC(year, month, 1) / 1000,
        monthKeys: { source_time: `${year}-${String(month + 1).padStart(2, "0")}`, observed_time: null },
        cover: item.cover, textAvailability: "available", anchorTopicId: "", times: [],
        author: { id: `demo-creator-${index}`, name: `${english ? "Creator" : "创作者"} ${String(index + 1).padStart(3, "0")}`, avatarURL: "" },
      };
    });
    const coverage = { source, worksTotal: works.length, worksInTopics: 0, uncategorizedWorks: works.length, worksNoText: 0, authorNames: works.length, authorIds: works.length, avatarRefs: 0, coverRefs: works.length };
    return {
      source, works, topics: [], topicMembership: [], edges: [], coverage,
      neighbors: catalog.neighbors.filter(edge => edge.source === source).map(edge => ({workId: ids.get(edge.workId)!, to: ids.get(edge.to)!, score: edge.score, shared: []})),
      timeCapabilities: { views: { current: true, source_time: true, observed_time: false } },
    };
  }).filter(scope => scope.works.length);
  return {
    schemaVersion: 3, analysisVersion: "website-sample-v1", inputVersion: "demo", snapshotId: `demo-${seed}`, inputRevision: `demo-${seed}`, ruleHash: "demo", snapshotKind: "demo", createdAt: "", scope: { accountId: "", source: "" },
    meta: { coverage: scopes.map(scope => scope.coverage) }, scopes,
  };
}
