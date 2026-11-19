/**
 * Steam Store API Integration
 * Using CORS proxy to bypass browser CORS restrictions
 */

// CORS proxy to bypass Steam API CORS restrictions
// Using allorigins as it's more reliable than corsproxy.io
const CORS_PROXY = 'https://api.allorigins.win/raw?url=';

const STEAM_SEARCH_API = 'https://store.steampowered.com/api/storesearch';
const STEAM_DETAILS_API = 'https://store.steampowered.com/api/appdetails';

/**
 * Search for games on Steam by name
 * @param {string} query - Search term
 * @returns {Promise<Array>} - Array of game results
 */
export const searchSteamGames = async (query) => {
  if (!query || query.trim().length < 2) {
    return [];
  }

  try {
    const steamUrl = `${STEAM_SEARCH_API}?term=${encodeURIComponent(query)}&cc=CN&l=schinese`;
    const response = await fetch(CORS_PROXY + encodeURIComponent(steamUrl));
    
    if (!response.ok) {
      throw new Error('Steam search failed');
    }

    const data = await response.json();
    
    // Return formatted results
    return (data.items || []).slice(0, 10).map(item => ({
      appid: item.id,
      name: item.name,
      thumbnail: item.tiny_image,
    }));
  } catch (error) {
    console.error('Steam search error:', error);
    return [];
  }
};

/**
 * Get detailed information about a Steam game
 * @param {number|string} appid - Steam App ID
 * @returns {Promise<Object>} - Game details formatted for AppDeck
 */
export const getSteamGameDetails = async (appid) => {
  try {
    const steamUrl = `${STEAM_DETAILS_API}?appids=${appid}&cc=CN&l=schinese`;
    const response = await fetch(CORS_PROXY + encodeURIComponent(steamUrl));
    
    if (!response.ok) {
      throw new Error('Failed to fetch game details');
    }

    const data = await response.json();
    const gameData = data[appid];
    
    if (!gameData || !gameData.success || !gameData.data) {
      throw new Error('Game data not available');
    }

    const game = gameData.data;
    
    // Strip HTML tags from description
    const stripHtml = (html) => {
      if (!html) return '';
      const tmp = document.createElement('div');
      tmp.innerHTML = html;
      return tmp.textContent || tmp.innerText || '';
    };

    // Generate launch parameter from game name using Base64 encoding
    // Supports all languages (Chinese, Japanese, Korean, etc.) and is decodable
    const generateLaunchParam = (name) => {
      try {
        // Unicode-safe base64 encoding
        // Convert string to UTF-8 bytes, then encode to base64
        const utf8Bytes = new TextEncoder().encode(name.trim());
        const binaryString = String.fromCharCode(...utf8Bytes);
        return btoa(binaryString);
      } catch (error) {
        console.error('Error encoding launch param:', error);
        // Fallback to simple slug if encoding fails
        return name.toLowerCase().trim().replace(/\s+/g, '-');
      }
    };

    // Extract genre names
    const genres = (game.genres || []).map(genre => genre.description);

    // Get the best images - prioritize larger sizes
    const iconUrl = game.header_image || game.capsule_image; // Medium size for icon (460x215)
    const bigPicUrl = (game.screenshots && game.screenshots[0]?.path_full) // First try screenshot (1920x1080)
      || game.background_raw // Fallback to background
      || game.header_image; // Last resort

    return {
      name: game.name,
      description: stripHtml(game.short_description || game.detailed_description),
      iconUrl: iconUrl,
      bigPicUrl: bigPicUrl,
      launchParam: generateLaunchParam(game.name),
      genres: genres,
    };
  } catch (error) {
    console.error('Steam game details error:', error);
    throw error;
  }
};

/**
 * Decode a launch parameter back to the original game name
 * @param {string} encoded - Base64 encoded launch parameter
 * @returns {string} - Original game name
 */
export const decodeLaunchParam = (encoded) => {
  try {
    // Decode base64 to UTF-8 bytes, then to string
    const binaryString = atob(encoded);
    const utf8Bytes = Uint8Array.from(binaryString, char => char.charCodeAt(0));
    return new TextDecoder().decode(utf8Bytes);
  } catch (error) {
    console.error('Error decoding launch param:', error);
    return encoded; // Return as-is if decoding fails
  }
};

export default {
  searchSteamGames,
  getSteamGameDetails,
  decodeLaunchParam,
};

