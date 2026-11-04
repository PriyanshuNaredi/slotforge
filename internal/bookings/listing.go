package bookings

type BookingListInput struct {
	Status *BookingStatus
	Limit  int
	Offset int
}

type BookingListResult struct {
	Bookings []Booking `json:"bookings"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	Total    int64     `json:"total"`
}
