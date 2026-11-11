package models

// AdminUser is created only by the application seed. There is no public signup.
type AdminUser struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
}
