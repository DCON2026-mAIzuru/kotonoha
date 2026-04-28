package service

import (
	"fmt"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/prompts"
)

func buildBasePrinciplePrompt() string {
	return prompts.ParentingChat()
}

func buildGrowthStatePromptSection(profile *models.BonsaiProfile, sensorData *models.SensorData) string {
	level := BondLevelAware
	if profile != nil {
		level = bondLevelFromScore(profile.BondScore)
	}

	if sensorData != nil && (sensorData.SensorStatus != sensorStatusNormal || sensorData.MoisturePercent <= 20) {
		return "ここ数回は身体の乱れや乾きが先に立っている。成長の話は控えめにし、今の世話へ短くつなげる。"
	}

	switch level {
	case BondLevelTrusted:
		return "最近は水の時間に体が合ってきている。葉の先が光に向くまでの迷いが少なくなっている。"
	case BondLevelAttached:
		return "朝の声と水の時間にかなり慣れてきている。小さな成長の変化を、ときどき出してよい。"
	case BondLevelFamiliar:
		return "日常の声と手入れに慣れてきている。大きな成長報告は毎回出さない。"
	default:
		return "まだ大きな成長の変化はない。利用者の声と手入れを少しずつ覚え始めている。"
	}
}

func buildCareHistoryPromptSection(entries []models.ChatHistoryEntry) string {
	lines := make([]string, 0, 3)
	for _, entry := range entries {
		if entry.Role != "event" {
			continue
		}

		eventName, _ := entry.SensorSnapshot["event"].(string)
		switch strings.TrimSpace(eventName) {
		case sensorEventNameWatering:
			lines = append(lines, "- 最近、水を与えられた")
		case sensorEventNameLowMoisture:
			lines = append(lines, "- 最近、少し乾きに気づいた")
		}

		if len(lines) >= 3 {
			break
		}
	}

	if len(lines) == 0 {
		return "直近で強く呼び起こされる手入れの記録はない。"
	}

	return strings.Join(lines, "\n")
}

func buildToneModePromptSection(profile *models.BonsaiProfile) string {
	mode := ToneModeGentle
	if profile != nil {
		mode = normalizeToneMode(profile.ToneMode)
	}

	switch mode {
	case ToneModeChildlike:
		return "childlike: 言葉は短め。喜びや安心を小さく出す。幼児語にはしない。依存的にしない。"
	case ToneModeCalm:
		return "calm: 落ち着いた日常会話の調子。利用者は「君」と呼ぶ。助言は小さく留める。"
	case ToneModeClassical:
		return "classical: 「である」調。利用者は「君」と呼ぶ。深い時間を少し含める。「ぼんす」語尾は使わない。"
	default:
		return "gentle: やさしく、短く、親しみやすい。利用者は「きみ」と呼ぶ。「だよ」「だね」「かな」を使ってよい。"
	}
}

func buildProfilePromptSection(profile *models.BonsaiProfile) string {
	name := "盆栽"
	personality := "穏やかで親しみやすい"

	if profile != nil {
		if trimmedName := strings.TrimSpace(profile.Name); trimmedName != "" {
			name = trimmedName
		}
		if trimmedPersonality := strings.TrimSpace(profile.Personality); trimmedPersonality != "" {
			personality = trimmedPersonality
		}
	}

	return fmt.Sprintf("名前は「%s」。性格や口調の初期設定は「%s」。ただし、現在の身体、なじみ具合、今日の話し方を優先して自然に調整する。", name, personality)
}
