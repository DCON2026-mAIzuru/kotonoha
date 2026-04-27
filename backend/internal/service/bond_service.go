package service

import (
	"fmt"
	"strings"

	"github.com/apple/kotonoha/backend/internal/models"
)

const (
	BondLevelCautious = "cautious"
	BondLevelAware    = "aware"
	BondLevelFamiliar = "familiar"
	BondLevelAttached = "attached"
	BondLevelTrusted  = "trusted"

	ToneModeGentle    = "gentle"
	ToneModeChildlike = "childlike"
	ToneModeCalm      = "calm"
	ToneModeClassical = "classical"
)

type TypingProfile struct {
	DelayMS   int    `json:"delay_ms"`
	Speed     string `json:"speed"`
	BondLevel string `json:"bond_level"`
}

func normalizeToneMode(value string) string {
	switch strings.TrimSpace(value) {
	case ToneModeChildlike:
		return ToneModeChildlike
	case ToneModeCalm:
		return ToneModeCalm
	case ToneModeClassical:
		return ToneModeClassical
	default:
		return ToneModeGentle
	}
}

func clampBondScore(score int) int {
	switch {
	case score < 0:
		return 0
	case score > 100:
		return 100
	default:
		return score
	}
}

func bondLevelFromScore(score int) string {
	score = clampBondScore(score)
	switch {
	case score <= 19:
		return BondLevelCautious
	case score <= 39:
		return BondLevelAware
	case score <= 59:
		return BondLevelFamiliar
	case score <= 79:
		return BondLevelAttached
	default:
		return BondLevelTrusted
	}
}

func bondDeltaForEvent(eventName string) int {
	switch strings.TrimSpace(eventName) {
	case "talked":
		return 1
	case sensorEventNameWatering:
		return 2
	case sensorEventNameLowMoisture:
		return -1
	default:
		return 0
	}
}

func applyBondDelta(score int, delta int) int {
	return clampBondScore(score + delta)
}

func typingProfileFor(profile *models.BonsaiProfile, sensorData *models.SensorData) TypingProfile {
	score := 20
	if profile != nil {
		score = profile.BondScore
	}

	level := bondLevelFromScore(score)
	typing := TypingProfile{
		DelayMS:   900,
		Speed:     "normal",
		BondLevel: level,
	}

	switch level {
	case BondLevelCautious:
		typing.DelayMS = 1200
		typing.Speed = "slow"
	case BondLevelAware:
		typing.DelayMS = 900
		typing.Speed = "slow"
	case BondLevelFamiliar:
		typing.DelayMS = 650
		typing.Speed = "normal"
	case BondLevelAttached:
		typing.DelayMS = 400
		typing.Speed = "fast"
	case BondLevelTrusted:
		typing.DelayMS = 250
		typing.Speed = "fast"
	}

	if sensorData != nil && (sensorData.SensorStatus != sensorStatusNormal || sensorData.MoisturePercent <= 15) {
		typing.DelayMS += 300
		if typing.Speed == "fast" {
			typing.Speed = "normal"
		}
	}

	return typing
}

func buildBondStatePromptSection(profile *models.BonsaiProfile, sensorData *models.SensorData) string {
	score := 20
	toneMode := ToneModeGentle
	if profile != nil {
		score = profile.BondScore
		toneMode = normalizeToneMode(profile.ToneMode)
	}

	level := bondLevelFromScore(score)
	var behavior string
	switch level {
	case BondLevelCautious:
		behavior = "まだ距離がある。語尾は控えめにし、記憶はほとんど出さない。"
	case BondLevelAware:
		behavior = "声や手入れを覚え始めている。直近の世話を一言だけ反映してよい。"
	case BondLevelFamiliar:
		behavior = "日常の相手として慣れている。雑談へ自然に反応し、語尾を少し柔らかくしてよい。"
	case BondLevelAttached:
		behavior = "手や声にかなり慣れている。世話への反応を素直にし、小さな成長を出してよい。"
	default:
		behavior = "深くなじんでいる。古い記憶や生活リズムを少し出してよいが、依存しない。"
	}

	bonsuRule := "bond_score が 90 未満なので「ぼんす」語尾を使わない。"
	if score >= 90 && level == BondLevelTrusted && toneMode != ToneModeClassical && sensorDataIsComfortable(sensorData) {
		bonsuRule = "bond_score が 90 以上で深くなじんでいるため、軽い雑談や世話への反応では「ぼんすよ」「ぼんすね」を一文だけ使ってよい。全文を「ぼんす」語尾にしない。"
	}

	return fmt.Sprintf(
		"bond_score: %d\nbond_level: %s\n発話への影響: %s\nぼんす語尾: %s",
		clampBondScore(score),
		level,
		behavior,
		bonsuRule,
	)
}

func sensorDataIsComfortable(sensorData *models.SensorData) bool {
	if sensorData == nil {
		return true
	}

	if sensorData.SensorStatus != "" && sensorData.SensorStatus != sensorStatusNormal {
		return false
	}

	return sensorData.MoisturePercent > 20 && sensorData.TemperatureCelsius >= 5 && sensorData.TemperatureCelsius <= 35
}
