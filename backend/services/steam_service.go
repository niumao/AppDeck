package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SteamService struct{}

func NewSteamService() *SteamService {
	return &SteamService{}
}

type SteamSearchResult struct {
	Name      string `json:"name"`
	IconURL   string `json:"icon_url"`
	BigPicURL string `json:"big_pic_url"`
}

func (s *SteamService) Search(query string) ([]SteamSearchResult, error) {
	if query == "" {
		return []SteamSearchResult{}, nil
	}

	searchURL := fmt.Sprintf("https://store.steampowered.com/api/storesearch/?term=%s&cc=CN&l=schinese",
		url.QueryEscape(query))

	resp, err := http.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("请求 Steam API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Steam API 返回错误状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp struct {
		Total int `json:"total"`
		Items []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Tiny string `json:"tiny_image"`
		} `json:"items"`
	}

	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	results := make([]SteamSearchResult, 0, len(apiResp.Items))
	for _, item := range apiResp.Items {
		iconURL := fmt.Sprintf("https://cdn.akamai.steamstatic.com/steam/apps/%d/capsule_sm_120.jpg", item.ID)
		bigPicURL := fmt.Sprintf("https://cdn.akamai.steamstatic.com/steam/apps/%d/header.jpg", item.ID)

		results = append(results, SteamSearchResult{
			Name:      item.Name,
			IconURL:   iconURL,
			BigPicURL: bigPicURL,
		})
	}

	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}

// ValidateSteamID validates a Steam ID format
func (s *SteamService) ValidateSteamID(steamID string) bool {
	if steamID == "" {
		return false
	}

	// Steam ID format: STEAM_X:Y:Z or SteamID64 (17 digits)
	// For simplicity, accept 17-digit numbers or STEAM_ prefix format
	if len(steamID) == 17 {
		// Check if all characters are digits
		for _, c := range steamID {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}

	// Check STEAM_X:Y:Z format
	if len(steamID) >= 11 && steamID[:6] == "STEAM_" {
		return true
	}

	return false
}
