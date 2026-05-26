package domain

import "time"

type ShipmentID string

type ShipmentStatus string

const (
	ShipmentStatusNew        ShipmentStatus = "NEW"
	ShipmentStatusPacked     ShipmentStatus = "PACKED"
	ShipmentStatusHandedOver ShipmentStatus = "HANDED_OVER"
	ShipmentStatusInTransit  ShipmentStatus = "IN_TRANSIT"
	ShipmentStatusDelivered  ShipmentStatus = "DELIVERED"
	ShipmentStatusFailed     ShipmentStatus = "FAILED"
)

type Shipment struct {
	ID      ShipmentID
	OrderID OrderID

	Carrier      string
	TrackingCode string
	LabelPath    string
	Status       ShipmentStatus

	CreatedAt time.Time
	UpdatedAt time.Time
}
