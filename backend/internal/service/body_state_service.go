package service

import (
	"strings"
	"time"

	"github.com/apple/kotonoha/backend/internal/models"
)

func buildBodyStatePromptSection(sensorData *models.SensorData) string {
	if sensorData == nil {
		return "まだ身体の様子は届いていない。感覚は少しぼんやりしている。"
	}

	lines := make([]string, 0, 4)
	switch strings.TrimSpace(sensorData.SensorStatus) {
	case sensorStatusStale:
		lines = append(lines, "新しい感覚がしばらく届いておらず、根元の様子が少し遠い。")
	case sensorStatusInvalid:
		lines = append(lines, "身体の感覚に乱れがあり、土や光の手触りをはっきり掴めない。")
	default:
		lines = append(lines, moistureBodyLine(sensorData.MoisturePercent))
	}

	lines = append(lines, lightBodyLine(sensorData.LightLux))
	lines = append(lines, temperatureBodyLine(sensorData.TemperatureCelsius))
	lines = append(lines, timeBodyLine(sensorData.RecordedAt))

	return strings.Join(lines, "\n")
}

func moistureBodyLine(moisture float64) string {
	switch {
	case moisture <= 15:
		return "根元はかなり乾いていて、言葉にも少し力が入りにくい。"
	case moisture <= 35:
		return "根元は乾き始めていて、もう少し水があると落ち着く。"
	case moisture <= 65:
		return "根元にはほどよい潤いがあり、静かに落ち着いている。"
	case moisture <= 85:
		return "根元はしっとりしていて、葉先まで安心が届いている。"
	default:
		return "根元には水がたっぷり巡っていて、少し満ち足りている。"
	}
}

func lightBodyLine(light float64) string {
	switch {
	case light <= 100:
		return "葉に届く光は弱く、周りは少し眠い。"
	case light <= 3000:
		return "葉に届く光はやわらかい。"
	case light <= 20000:
		return "葉には十分な光が当たっている。"
	default:
		return "葉に当たる光は強く、少し身構えている。"
	}
}

func temperatureBodyLine(temp float64) string {
	switch {
	case temp < 8:
		return "鉢の温度は低く、動きはゆっくりになる。"
	case temp > 32:
		return "鉢の温度は高く、少し息が浅い。"
	default:
		return "鉢の温度は落ち着いている。"
	}
}

func timeBodyLine(recordedAt time.Time) string {
	if recordedAt.IsZero() {
		return "最後に身体を確かめた時刻はわからない。"
	}

	return "身体の感覚は、最近届いたものとして扱う。"
}
