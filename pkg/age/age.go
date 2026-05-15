package age

import "time"

// YearsAt returns full years between birthDate and now (Colombia age gate uses calendar age).
func YearsAt(birthDate, now time.Time) int {
	if birthDate.IsZero() {
		return 0
	}
	y := now.Year() - birthDate.Year()
	if now.YearDay() < birthDate.YearDay() {
		y--
	}
	return y
}

// IsAdult returns true when the person is at least 18 years old at now.
func IsAdult(birthDate, now time.Time) bool {
	return YearsAt(birthDate, now) >= 18
}
