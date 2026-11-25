export interface VideoQualityOption {
  id: string;
  label: string;
  videos: string[];
}

export const QUALITY_STORAGE_KEY = "hoard-video-quality";

export interface ResolvedVideoQuality {
  activeId: string;
  activeLabel: string;
  videos: string[];
}

function dedupeUrls(groups: readonly (readonly string[])[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const group of groups) {
    for (const url of group) {
      if (!url || seen.has(url)) continue;
      seen.add(url);
      out.push(url);
    }
  }
  return out;
}

export function resolveVideoQuality(
  qualities: VideoQualityOption[] | undefined,
  fallbackVideos: string[],
  preferred: string,
): ResolvedVideoQuality {
  if (!qualities || qualities.length === 0) {
    return {
      activeId: "",
      activeLabel: "",
      videos: dedupeUrls([fallbackVideos]),
    };
  }

  let selected: VideoQualityOption | undefined;
  const pref = preferred.trim();

  if (!pref || pref === "auto") {
    selected = qualities[0];
  } else {
    selected = qualities.find((q) => q.id === pref);
    if (!selected) {
      const target = Number(pref);
      if (Number.isFinite(target) && target > 0) {
        const lowerOrEqual = qualities
          .filter((q) => {
            const rank = Number(q.id);
            return Number.isFinite(rank) && rank <= target;
          })
          .sort((a, b) => Number(b.id) - Number(a.id));
        if (lowerOrEqual.length > 0) {
          selected = lowerOrEqual[0];
        } else {
          const byRankAsc = [...qualities].sort((a, b) => Number(a.id) - Number(b.id));
          selected = byRankAsc[0];
        }
      } else {
        selected = qualities[0];
      }
    }
  }

  return {
    activeId: selected.id,
    activeLabel: selected.label,
    videos: dedupeUrls([selected.videos, fallbackVideos]),
  };
}
