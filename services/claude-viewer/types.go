package claudeviewer

import (
	"math"
	"time"
	"unicode"
)

const (
	AverageReadingSpeed    = 200
	DefaultReadingSpeedWPM = 200
)

// PlanSummary represents a summary of a plan for listing.
type PlanSummary struct {
	ID          int64
	FileName    string
	Title       string
	CreatedAt   time.Time
	ModifiedAt  time.Time
	FileSize    int64
	ReadingTime int
}

// PlanDetail represents detailed plan information with rendered HTML.
type PlanDetail struct {
	PlanSummary
	Content      string
	RenderedHTML string
}

// SettingValues holds all possible value types for a setting.
type SettingValues struct {
	StringValue   *string    `json:"stringValue,omitempty"`
	NumberValue   *float64   `json:"numberValue,omitempty"`
	BooleanValue  *bool      `json:"booleanValue,omitempty"`
	DateTimeValue *time.Time `json:"dateTimeValue,omitempty"`
}

func CalculateReadingTime(wordCount int) int {
	minutes := float64(wordCount) / float64(AverageReadingSpeed)
	return max(1, int(math.Ceil(minutes)))
}

// CalculateReadingTimeWithWPM calculates reading time based on word count and custom WPM.
func (s *Service) CalculateReadingTimeWithWPM(wordCount, wpm int) int {
	if wpm <= 0 {
		wpm = DefaultReadingSpeedWPM
	}
	minutes := float64(wordCount) / float64(wpm)
	return max(1, int(math.Ceil(minutes)))
}

func CountWords(content string) int {
	if content == "" {
		return 0
	}

	words := 0
	inWord := false

	for _, r := range content {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if inWord {
				words++
				inWord = false
			}
		} else if unicode.IsLetter(r) || unicode.IsNumber(r) {
			inWord = true
		}
	}

	if inWord {
		words++
	}

	return words
}
