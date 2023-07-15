package models

import (
	"time"
)

type SceneMarker struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	Seconds      float64   `json:"seconds"`
	PrimaryTagID int       `json:"primary_tag_id"`
	SceneID      int       `json:"scene_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func NewSceneMarker() SceneMarker {
	currentTime := time.Now()
	return SceneMarker{
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}
}
