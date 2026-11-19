/**
 * RetroArch Thumbnail Search via libretro-thumbnails
 * Uses GitHub Git Trees API to fetch full game lists, thumbnails.libretro.com CDN for images.
 */

// Map RetroArch core names to libretro-thumbnails repo/system names (spaces for CDN)
const CORE_TO_SYSTEM = {
  snes9x: 'Nintendo - Super Nintendo Entertainment System',
  mgba: 'Nintendo - Game Boy Advance',
  nestopia: 'Nintendo - Nintendo Entertainment System',
  genesis_plus_gx: 'Sega - Mega Drive - Genesis',
  pcsx_rearmed: 'Sony - PlayStation',
  ppsspp: 'Sony - PlayStation Portable',
  mupen64plus_next: 'Nintendo - Nintendo 64',
  fbneo: 'FBNeo - Arcade Games',
  fbalpha2012_neogeo: 'FBNeo - Arcade Games',
  gambatte: 'Nintendo - Game Boy',
  fceumm: 'Nintendo - Nintendo Entertainment System',
  picodrive: 'Sega - 32X',
  mednafen_pce_fast: 'NEC - PC Engine - TurboGrafx 16',
  mednafen_ngp: 'SNK - Neo Geo Pocket',
  melonds: 'Nintendo - Nintendo DS',
};

const BOXARTS_PREFIX = 'Named_Boxarts/';
const CACHE_VERSION = 'v2';

/**
 * Fetch the full game list for a given core from GitHub Git Trees API.
 * Uses recursive tree endpoint to bypass the 1000-item Contents API limit.
 * Results are cached in sessionStorage per core.
 * @param {string} core - RetroArch core name (e.g. 'snes9x')
 * @returns {Promise<string[]>} - Array of game names (without .png extension)
 */
export const fetchGameList = async (core) => {
  const system = CORE_TO_SYSTEM[core];
  if (!system) {
    return [];
  }

  // Check sessionStorage cache
  const cacheKey = `retroarch_${CACHE_VERSION}_${core}`;
  const cached = sessionStorage.getItem(cacheKey);
  if (cached) {
    return JSON.parse(cached);
  }

  // GitHub repo names use underscores instead of spaces
  const repoName = system.replace(/ /g, '_');
  const url = `https://api.github.com/repos/libretro-thumbnails/${encodeURIComponent(repoName)}/git/trees/master?recursive=1`;
  const response = await fetch(url, {
    headers: { Accept: 'application/vnd.github.v3+json' },
  });

  if (!response.ok) {
    throw new Error(`GitHub API error: ${response.status}`);
  }

  const data = await response.json();
  const names = (data.tree || [])
    .filter((t) => t.path.startsWith(BOXARTS_PREFIX) && t.path.endsWith('.png'))
    .map((t) => t.path.slice(BOXARTS_PREFIX.length, -4));

  // Cache in sessionStorage
  sessionStorage.setItem(cacheKey, JSON.stringify(names));
  return names;
};

/**
 * Search RetroArch games by core and query string.
 * @param {string} core - RetroArch core name
 * @param {string} query - Search term
 * @returns {Promise<Array<{name: string, displayName: string, thumbnail: string, boxartUrl: string, snapUrl: string}>>}
 */
export const searchRetroarchGames = async (core, query) => {
  if (!core || !query || query.trim().length < 2) {
    return [];
  }

  const games = await fetchGameList(core);
  const system = CORE_TO_SYSTEM[core];
  const lowerQuery = query.toLowerCase();

  return games
    .filter((name) => name.toLowerCase().includes(lowerQuery))
    .slice(0, 10)
    .map((name) => {
      const encodedName = encodeURIComponent(name);
      return {
        name,
        displayName: name,
        thumbnail: `https://thumbnails.libretro.com/${encodeURIComponent(system)}/Named_Boxarts/${encodedName}.png`,
        boxartUrl: `https://thumbnails.libretro.com/${encodeURIComponent(system)}/Named_Boxarts/${encodedName}.png`,
        snapUrl: `https://thumbnails.libretro.com/${encodeURIComponent(system)}/Named_Snaps/${encodedName}.png`,
      };
    });
};

export default {
  fetchGameList,
  searchRetroarchGames,
};
