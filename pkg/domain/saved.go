package domain

import "time"

type SavedItem struct {
	ID      string    `json:"-" bson:"_id"`
	UserID  string    `json:"-" bson:"user_id"`
	Product *Product  `json:"product" bson:"product"`
	SavedAt time.Time `json:"savedAt" bson:"saved_at"`
}
