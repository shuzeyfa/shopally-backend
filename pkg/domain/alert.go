package domain

import "time"

type Alert struct {
	ID             string     `json:"alertId" bson:"_id"`
	UserID         string     `json:"userId" bson:"user_id"`
	ProductID      string     `json:"productId" bson:"product_id"`
	TargetPrice    float64    `json:"targetPrice" bson:"target_price"`
	Currency       string     `json:"currency" bson:"currency"`
	IsActive       bool       `json:"isActive" bson:"is_active"`
	TriggeredAt    *time.Time `json:"triggeredAt,omitempty" bson:"triggered_at,omitempty"`
	TriggeredPrice float64    `json:"triggeredPrice,omitempty" bson:"triggered_price,omitempty"`
}
