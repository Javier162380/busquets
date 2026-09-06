package busquets

import "github.com/Javier162380/busquets/services/busquets/dto"

func toTag(t dto.Tag) Tag {
	return Tag{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Color:       t.Color,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func toTags(ts []dto.Tag) []Tag {
	out := make([]Tag, len(ts))
	for i, t := range ts {
		out[i] = toTag(t)
	}
	return out
}

func toComment(c dto.Comment) Comment {
	return Comment{
		ID:        c.ID,
		PlanID:    c.PlanID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func toComments(cs []dto.Comment) []Comment {
	out := make([]Comment, len(cs))
	for i, c := range cs {
		out[i] = toComment(c)
	}
	return out
}

// toPlanVersion maps a dto.PlanVersion row to the domain PlanVersion type.
func toPlanVersion(v dto.PlanVersion) PlanVersion {
	return PlanVersion{
		ID:            v.ID,
		PlanID:        v.PlanID,
		VersionNumber: v.VersionNumber,
		FilePath:      v.FilePath,
		Content:       v.Content,
		WordCount:     v.WordCount,
		CreatedAt:     v.CreatedAt,
	}
}
