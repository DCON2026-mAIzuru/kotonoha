package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultVoicevoxSpeakerID = 13
	defaultVoicevoxSpeed     = 1.0
	minVoicevoxSpeed         = 0.5
	maxVoicevoxSpeed         = 2.0
	maxVoicevoxTextRunes     = 1200
)

type VoicevoxService struct {
	engineURL   string
	speakerID   int
	speakerName string
	styleName   string
	speedScale  float64
	client      *http.Client
}

type voicevoxAudioQuery map[string]any

type voicevoxSpeaker struct {
	Name   string               `json:"name"`
	Styles []voicevoxStyleEntry `json:"styles"`
}

type voicevoxStyleEntry struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

func NewVoicevoxService(engineURL string, speakerID string, speakerName string, styleName string, speedScale string, timeout time.Duration) *VoicevoxService {
	normalizedEngineURL := strings.TrimRight(strings.TrimSpace(engineURL), "/")
	if normalizedEngineURL == "" {
		normalizedEngineURL = "http://127.0.0.1:50021"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	parsedSpeakerID, err := strconv.Atoi(strings.TrimSpace(speakerID))
	if err != nil || parsedSpeakerID <= 0 {
		parsedSpeakerID = defaultVoicevoxSpeakerID
	}
	parsedSpeedScale, err := strconv.ParseFloat(strings.TrimSpace(speedScale), 64)
	if err != nil || parsedSpeedScale <= 0 {
		parsedSpeedScale = defaultVoicevoxSpeed
	}
	parsedSpeedScale = clampFloat(parsedSpeedScale, minVoicevoxSpeed, maxVoicevoxSpeed)

	return &VoicevoxService{
		engineURL:   normalizedEngineURL,
		speakerID:   parsedSpeakerID,
		speakerName: strings.TrimSpace(speakerName),
		styleName:   strings.TrimSpace(styleName),
		speedScale:  parsedSpeedScale,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *VoicevoxService) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("VOICEVOXサービスが初期化されていません")
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, errors.New("読み上げるテキストが空です")
	}
	if utf8.RuneCountInString(trimmed) > maxVoicevoxTextRunes {
		return nil, fmt.Errorf("読み上げテキストは%d文字以下にしてください", maxVoicevoxTextRunes)
	}

	speakerID := s.speakerID
	if s.speakerName != "" {
		if resolvedID, err := s.resolveSpeakerID(ctx); err == nil {
			speakerID = resolvedID
		}
	}

	audioQuery, err := s.audioQuery(ctx, trimmed, speakerID)
	if err != nil {
		return nil, err
	}
	audioQuery["speedScale"] = s.speedScale

	audio, err := s.synthesis(ctx, audioQuery, speakerID)
	if err != nil {
		return nil, err
	}
	if len(audio) == 0 {
		return nil, errors.New("VOICEVOX Engineから空の音声が返されました")
	}

	return audio, nil
}

func (s *VoicevoxService) resolveSpeakerID(ctx context.Context) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.engineURL+"/speakers", nil)
	if err != nil {
		return 0, err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("VOICEVOX speakers API error: status=%d", response.StatusCode)
	}

	var speakers []voicevoxSpeaker
	if err := json.NewDecoder(response.Body).Decode(&speakers); err != nil {
		return 0, err
	}

	for _, speaker := range speakers {
		if speaker.Name != s.speakerName {
			continue
		}
		if s.styleName == "" && len(speaker.Styles) > 0 {
			return speaker.Styles[0].ID, nil
		}
		for _, style := range speaker.Styles {
			if style.Name == s.styleName {
				return style.ID, nil
			}
		}
	}

	return 0, fmt.Errorf("VOICEVOX speaker not found: speaker=%s style=%s", s.speakerName, s.styleName)
}

func (s *VoicevoxService) audioQuery(ctx context.Context, text string, speakerID int) (voicevoxAudioQuery, error) {
	endpoint := fmt.Sprintf("%s/audio_query?text=%s&speaker=%d", s.engineURL, url.QueryEscape(text), speakerID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return nil, errors.New("VOICEVOX Engineに接続できません。Docker DesktopまたはVOICEVOX Engineを起動してください")
	}
	defer response.Body.Close()

	payload, err := readLimitedVoicevoxResponse(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("VOICEVOX audio_query が失敗しました: status=%d body=%s", response.StatusCode, string(payload))
	}

	var query voicevoxAudioQuery
	if err := json.Unmarshal(payload, &query); err != nil {
		return nil, fmt.Errorf("VOICEVOX audio_query の応答を解析できません: %w", err)
	}

	return query, nil
}

func (s *VoicevoxService) synthesis(ctx context.Context, audioQuery voicevoxAudioQuery, speakerID int) ([]byte, error) {
	queryPayload, err := json.Marshal(audioQuery)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/synthesis?speaker=%d", s.engineURL, speakerID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(queryPayload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "audio/wav")

	response, err := s.client.Do(request)
	if err != nil {
		return nil, errors.New("VOICEVOX Engineで音声を生成できません。Docker DesktopまたはVOICEVOX Engineの起動状態を確認してください")
	}
	defer response.Body.Close()

	payload, err := readLimitedVoicevoxResponse(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("VOICEVOX synthesis が失敗しました: status=%d body=%s", response.StatusCode, string(payload))
	}

	return payload, nil
}

func readLimitedVoicevoxResponse(reader io.Reader) ([]byte, error) {
	const maxVoicevoxResponseBytes = 32 * 1024 * 1024
	payload, err := io.ReadAll(io.LimitReader(reader, maxVoicevoxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxVoicevoxResponseBytes {
		return nil, errors.New("VOICEVOX Engineの応答が大きすぎます")
	}

	return payload, nil
}
