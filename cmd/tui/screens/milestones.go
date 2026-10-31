package screens

import (
	"strings"

	"github.com/Javier162380/busquets/services/busquets"
)

// milestoneBadge is the short label a milestone gets on a list row.
var milestoneBadge = map[busquets.VersionMilestone]string{
	busquets.MilestoneLLMGenerated:             "LLM",
	busquets.MilestoneHumanReview:              "Review",
	busquets.MilestonePlanImplementationStart:  "Impl▶",
	busquets.MilestonePlanImplementationFinish: "Impl■",
	busquets.MilestonePlanAbandoned:            "Abandoned",
}

// milestoneBadges renders milestones for a list row, e.g. "◆ Review, Impl▶"; "" when none.
func milestoneBadges(milestones []busquets.VersionMilestone) string {
	if len(milestones) == 0 {
		return ""
	}
	badges := make([]string, len(milestones))
	for i, milestone := range milestones {
		badges[i] = milestoneBadge[milestone]
	}
	return "◆ " + strings.Join(badges, ", ")
}

// milestoneList renders milestones in full for the metadata modal; "-" when none.
func milestoneList(milestones []busquets.VersionMilestone) string {
	if len(milestones) == 0 {
		return "-"
	}
	names := make([]string, len(milestones))
	for i, milestone := range milestones {
		names[i] = string(milestone)
	}
	return strings.Join(names, ", ")
}
