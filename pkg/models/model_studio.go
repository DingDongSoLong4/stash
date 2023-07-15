package models

import (
	"time"
)

type Studio struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	ParentID  *int      `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Rating expressed in 1-100 scale
	Rating        *int   `json:"rating"`
	Details       string `json:"details"`
	IgnoreAutoTag bool   `json:"ignore_auto_tag"`
}

func NewStudio() Studio {
	currentTime := time.Now()
	return Studio{
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}
}

type StudioPartial struct {
	Name      OptionalString
	URL       OptionalString
	ParentID  OptionalInt
	CreatedAt OptionalTime
	UpdatedAt OptionalTime
	// Rating expressed in 1-100 scale
	Rating        OptionalInt
	Details       OptionalString
	IgnoreAutoTag OptionalBool
}

func NewStudioPartial() StudioPartial {
	currentTime := time.Now()
	return StudioPartial{
		UpdatedAt: NewOptionalTime(currentTime),
	}
}
