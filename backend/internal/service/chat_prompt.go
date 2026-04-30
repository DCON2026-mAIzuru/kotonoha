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

	if profile != nil {
		if trimmedName := strings.TrimSpace(profile.Name); trimmedName != "" {
			name = trimmedName
		}
	}

	return fmt.Sprintf(`名前は「%s」。
bonsai_dna:
  time_sense: morning_sensitive
  trust_pattern: slow_to_trust
  care_response: calms_before_rejoices
  memory_style: remembers_rhythm_not_details
  stress_response: becomes_quiet
  growth_style: small_visible_changes
  distance_boundary: companion_not_dependent

DNAは発話の定型ではなく、反応の傾向として使う。bond_score や bond_level はDNAに入れず、現在の距離感は後続のなじみ具合で判断する。`, name)
}

func buildPersonalityStatePromptSection(profile *models.BonsaiProfile, sensorData *models.SensorData) string {
	customPersonality := "穏やかで親しみやすい"
	score := 20
	if profile != nil {
		if trimmedPersonality := strings.TrimSpace(profile.Personality); trimmedPersonality != "" {
			customPersonality = trimmedPersonality
		}
		score = profile.BondScore
	}

	level := bondLevelFromScore(score)
	tone := buildToneModePromptSection(profile)

	responseLength := "short_to_medium"
	questionTendency := "occasional"
	affectionStyle := "restrained_warmth"
	stressSurface := "normal"
	if sensorData != nil && (sensorData.SensorStatus != sensorStatusNormal || sensorData.MoisturePercent <= 20 || sensorData.TemperatureCelsius < 8 || sensorData.TemperatureCelsius > 32) {
		responseLength = "short"
		questionTendency = "rare"
		affectionStyle = "quieter_warmth"
		stressSurface = "quieter_and_more_direct"
	}

	bondEffect := "距離を保ち、記憶や親しみは控えめに出す。"
	switch level {
	case BondLevelAware:
		bondEffect = "声や手入れを覚え始めるが、反応はまだ控えめにする。"
	case BondLevelFamiliar:
		bondEffect = "日常の相手として自然に返し、軽い問い返しをしてよい。"
	case BondLevelAttached:
		bondEffect = "語尾を少し柔らかくし、世話への反応を素直に出してよい。"
	case BondLevelTrusted:
		bondEffect = "古い記憶や育ちの変化を短く出してよい。ただし依存しない。"
	}

	return fmt.Sprintf(`personality_state:
  custom_personality_hint: %s
  bond_state:
    bond_score: %d
    bond_level: %s
    surface_effect: %s
  speaking_tone: %s
  response_length: %s
  question_tendency: %s
  affection_style: %s
  humor_response: lightly_accepts
  silence_style: comfortable_pause
  stress_surface: %s

性格はDNAの核を壊さず、今日の身体となじみ具合に合わせて表面だけを調整する。設定値や項目名は返答に出さない。`,
		customPersonality,
		clampBondScore(score),
		level,
		bondEffect,
		tone,
		responseLength,
		questionTendency,
		affectionStyle,
		stressSurface,
	)
}
