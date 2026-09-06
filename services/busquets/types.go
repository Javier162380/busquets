package busquets

import (
	"math"
	"time"
	"unicode"
)

// Tag represents a tag that can be associated with plans.
type Tag struct {
	ID          int64
	Name        string
	Description *string
	Color       *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Comment represents a user-written annotation on a plan.
type Comment struct {
	ID        int64
	PlanID    int64
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	AverageReadingSpeed    = 200
	DefaultReadingSpeedWPM = 200
)

// PlanSummary represents a summary of a plan for listing.
type PlanSummary struct {
	ID           int64
	FileName     string
	SyncSource   string
	SyncLabel    string
	Title        string
	CreatedAt    time.Time
	ModifiedAt   time.Time
	FileSize     int64
	ReadingTime  int
	Tags         []Tag
	CommentCount int
}

// PlanDetail represents detailed plan information with rendered HTML.
type PlanDetail struct {
	PlanSummary
	// FilePath is the absolute path of the plan's mirror copy in the viewer
	// directory (as stored in the DB). Used by delete so the caller sends the
	// authoritative path rather than the service re-deriving it.
	FilePath string
	Content  string
}

// SettingValues holds all possible value types for a setting.
type SettingValues struct {
	StringValue   *string    `json:"stringValue,omitempty"`
	NumberValue   *float64   `json:"numberValue,omitempty"`
	BooleanValue  *bool      `json:"booleanValue,omitempty"`
	DateTimeValue *time.Time `json:"dateTimeValue,omitempty"`
}

type PlanVersion struct {
	ID            int64
	PlanID        int64
	VersionNumber int64
	FilePath      string
	Content       string
	WordCount     int64
	CreatedAt     time.Time
}

type PlanVersionDetail struct {
	PlanVersion
	ReadingTime int
	Tags        []Tag
}

// VersionDiff is the result of comparing two plan versions.
type VersionDiff struct {
	Diff string
	From PlanVersion
	To   PlanVersion
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
