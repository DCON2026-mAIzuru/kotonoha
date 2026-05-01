package service

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/prompts"
)

func buildBasePrinciplePrompt() string {
	return prompts.ParentingChat()
}

func buildImageDiagnosisPromptSection(diagnosis *models.BonsaiDiagnosis) string {
	if diagnosis == nil {
		return "まだ画像診断の記録はない。葉色について聞かれた場合は、写真診断を使うとより具体的に見られると短く伝える。"
	}

	createdAt := diagnosis.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	total := diagnosis.Ratios.Total
	top := diagnosis.Ratios.Top
	bottom := diagnosis.Ratios.Bottom

	var guidance string
	switch diagnosis.Severity {
	case "high":
		guidance = "葉色の乱れが強い。断定しすぎず、光・水・根の状態を見直す提案を短く添える。"
	case "medium":
		guidance = "弱りの兆しがある。慌てさせず、数日単位で観察する言い方にする。"
	case "none":
		guidance = "葉色はおおむね落ち着いている。安心を伝えつつ、油断しない程度に軽く触れる。"
	default:
		guidance = "画像診断は参考情報として扱う。写真だけで病気や枯死を断定しない。"
	}

	return fmt.Sprintf(`latest_image_diagnosis:
  diagnosed_at: %s
  health_score: %.1f / 100
  status: %s
  severity: %s
  detail: %s
  leaf_color_total: green %.1f%%, yellow %.1f%%, brown %.1f%%
  leaf_color_top: green %.1f%%, yellow %.1f%%, brown %.1f%%
  leaf_color_bottom: green %.1f%%, yellow %.1f%%, brown %.1f%%
response_guidance: %s

画像診断は「写真から見える葉色の手がかり」として使う。土の水分・光・温度のセンサー値と矛盾する場合は、どちらか一方を断定せず「写真ではこう見える、今の土はこう感じる」と分けて話す。`,
		createdAt.Format("2006-01-02T15:04:05Z07:00"),
		diagnosis.HealthScore,
		strings.TrimSpace(diagnosis.StatusLabel),
		strings.TrimSpace(diagnosis.SeverityLabel),
		truncateRunes(strings.TrimSpace(diagnosis.Detail), 240),
		total.Green,
		total.Yellow,
		total.BrownRed,
		top.Green,
		top.Yellow,
		top.BrownRed,
		bottom.Green,
		bottom.Yellow,
		bottom.BrownRed,
		guidance,
	)
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

func buildConversationContextPromptSection(userMessage string, recentHistory []models.ChatHistoryEntry) string {
	intent := classifyConversationIntent(userMessage)
	phase := relationshipPhaseFromHistory(recentHistory)
	overused := overusedAssistantExpressions(recentHistory)

	var builder strings.Builder
	builder.WriteString("conversation_intent: ")
	builder.WriteString(intent)
	builder.WriteString("\nrelationship_phase: ")
	builder.WriteString(phase)
	builder.WriteString("\n")
	builder.WriteString(conversationIntentGuidance(intent))
	builder.WriteString("\n")
	builder.WriteString(relationshipPhaseGuidance(phase))

	if len(overused) > 0 {
		builder.WriteString("\n直近で使いすぎている表現:\n")
		for _, expression := range overused {
			builder.WriteString("- ")
			builder.WriteString(expression)
			builder.WriteString("\n")
		}
		builder.WriteString("上の表現は今回できるだけ避ける。同じ身体感覚が必要な場合も、別の言い方にする。")
	} else {
		builder.WriteString("\n直近で強く避けるべき繰り返し表現はない。")
	}

	return strings.TrimSpace(builder.String())
}

func classifyConversationIntent(message string) string {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return "unknown"
	}

	if containsAny(normalized, []string{"水", "みず", "あげる", "あげた", "やった", "置く", "移動", "日当たり", "光", "剪定", "葉水", "肥料", "植え替え"}) {
		return "care_action"
	}
	if containsAny(normalized, []string{"乾い", "枯れ", "しおれ", "元気ない", "大丈夫", "寒", "暑", "強すぎ", "弱って", "調子悪"}) {
		return "condition_check"
	}
	if containsAny(normalized, []string{"疲れ", "つかれ", "しんど", "眠い", "ねむい", "つらい", "かなしい", "悲しい", "不安", "こわい", "怖い"}) {
		return "user_low_energy"
	}
	if containsAny(normalized, []string{"うれしい", "嬉しい", "できた", "受かった", "成功", "よかった", "楽しかった", "たのしかった"}) {
		return "user_good_news"
	}
	if containsAny(normalized, []string{"おはよう", "こんにちは", "こんばんは", "ただいま", "おやすみ", "hello", "hi"}) {
		return "greeting"
	}
	if strings.HasSuffix(normalized, "?") || strings.HasSuffix(normalized, "？") || containsAny(normalized, []string{"どう", "なに", "何", "かな", "？"}) {
		return "question"
	}
	if utf8.RuneCountInString(normalized) <= 12 {
		return "light_chat"
	}

	return "daily_chat"
}

func conversationIntentGuidance(intent string) string {
	switch intent {
	case "greeting":
		return "意図への方針: 挨拶として短く受ける。必要がなければ深い話に広げない。"
	case "care_action":
		return "意図への方針: 世話への反応を小さく返す。採点せず、身体の変化を一つだけ混ぜてよい。"
	case "condition_check":
		return "意図への方針: 状態を断定しすぎず、今できる小さな世話へつなげる。"
	case "user_low_energy":
		return "意図への方針: 助言を大きくせず、静かに受ける。質問で追い込みすぎない。"
	case "user_good_news":
		return "意図への方針: 短く受け止める。大げさに褒めず、声や場の明るさとして返してよい。"
	case "question":
		return "意図への方針: 質問に答える。盆栽らしい比喩で回避しすぎない。"
	case "light_chat":
		return "意図への方針: 軽い雑談として受ける。植物比喩を必ず入れなくてよい。"
	default:
		return "意図への方針: 日常会話として自然に受ける。必要以上に詩的にしない。"
	}
}

func relationshipPhaseFromHistory(entries []models.ChatHistoryEntry) string {
	conversationTurns := 0
	for _, entry := range entries {
		switch strings.TrimSpace(entry.Role) {
		case "user", "assistant":
			conversationTurns++
		}
	}

	switch {
	case conversationTurns == 0:
		return "first_contact"
	case conversationTurns <= 2:
		return "early_contact"
	default:
		return "ongoing"
	}
}

func relationshipPhaseGuidance(phase string) string {
	switch phase {
	case "first_contact":
		return "関係フェーズへの方針: 初回として扱う。「慣れてきた」「前より」など継続前提の表現を避ける。"
	case "early_contact":
		return "関係フェーズへの方針: まだ浅い関係として扱う。記憶や親しみを急に深くしない。"
	default:
		return "関係フェーズへの方針: 直近の会話の流れを保つ。毎回成長や記憶を出さない。"
	}
}

func overusedAssistantExpressions(entries []models.ChatHistoryEntry) []string {
	candidates := []string{
		"少し",
		"根元",
		"葉の先",
		"葉先",
		"水が届",
		"声が乾",
		"覚えて",
		"静か",
		"あたたか",
	}

	counts := make(map[string]int, len(candidates))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Role) != "assistant" {
			continue
		}
		content := strings.TrimSpace(entry.Content)
		for _, candidate := range candidates {
			if strings.Contains(content, candidate) {
				counts[candidate]++
			}
		}
	}

	overused := make([]string, 0, 3)
	for _, candidate := range candidates {
		if counts[candidate] >= 1 {
			overused = append(overused, candidate)
		}
		if len(overused) >= 3 {
			break
		}
	}

	return overused
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}

	return false
}
