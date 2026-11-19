package services

import (
	"errors"

	"appdeck/database"
	"appdeck/models"

	"gorm.io/gorm"
)

type NotebookService struct{}

var notebookServiceInstance *NotebookService

// NewNotebookService returns the singleton notebook service instance
func NewNotebookService() *NotebookService {
	if notebookServiceInstance == nil {
		notebookServiceInstance = &NotebookService{}
	}
	return notebookServiceInstance
}

// GetNote retrieves the notebook content for an admin user.
// Returns "" (not an error) if the user has no note yet.
func (s *NotebookService) GetNote(userID string) (string, error) {
	var note models.AdminNotebook
	err := database.DB.Where("user_id = ?", userID).First(&note).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return note.Content, nil
}

// SaveNote creates or updates the notebook content for an admin user,
// returning the saved record (so callers can read back UpdatedAt).
func (s *NotebookService) SaveNote(userID string, content string) (*models.AdminNotebook, error) {
	var note models.AdminNotebook
	err := database.DB.Where("user_id = ?", userID).First(&note).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			note = models.AdminNotebook{
				UserID:  userID,
				Content: content,
			}
			if err := database.DB.Create(&note).Error; err != nil {
				return nil, err
			}
			return &note, nil
		}
		return nil, err
	}

	note.Content = content
	if err := database.DB.Save(&note).Error; err != nil {
		return nil, err
	}
	return &note, nil
}
