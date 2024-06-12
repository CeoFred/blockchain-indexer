package repository

import (
	"fmt"

	"github.com/CeoFred/gin-boilerplate/internal/models"
	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type EventLogInterface interface {
	Find(id uuid.UUID) (*models.EventLog, error)
	Exists(id uuid.UUID) (bool, error)
	Where(condition, value string) ([]*models.EventLog, error)
	Create(contract *models.EventLog) error
	Save(contract *models.EventLog) (*models.EventLog, error)
	RawCount(q string, count *int64) error
	QueryWithArgs(q string, args ...interface{}) (*models.EventLog, error)
	QueryRecordsWithArgs(q string, args ...interface{}) ([]*models.EventLog, error)
	RawSmartSelect(q string, res interface{}, args ...interface{}) error
}

type EventLogRepository struct {
	database *gorm.DB
}

func NewEventLogRepository(db *gorm.DB) EventLogInterface {
	return &EventLogRepository{
		database: db,
	}
}

func (a *EventLogRepository) Find(id uuid.UUID) (*models.EventLog, error) {
	var contract models.EventLog
	err := a.database.First(&contract, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (a *EventLogRepository) Exists(id uuid.UUID) (bool, error) {
	var count int64
	err := a.database.Model(&models.EventLog{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *EventLogRepository) Where(condition, value string) ([]*models.EventLog, error) {
	var contracts []*models.EventLog
	err := a.database.Raw(fmt.Sprintf(`SELECT * FROM event_logs WHERE %s = ?`, condition), value).Scan(&contracts).Error
	if err != nil {
		return nil, err
	}
	return contracts, nil
}

func (a *EventLogRepository) Create(contract *models.EventLog) error {
	return a.database.Create(contract).Error
}

func (a *EventLogRepository) Save(contract *models.EventLog) (*models.EventLog, error) {
	err := a.database.Model(contract).Where("id = ?", contract.ID).Updates(contract).Error
	if err != nil {
		return nil, err
	}
	return contract, nil
}

func (a *EventLogRepository) RawCount(q string, count *int64) error {
	return a.database.Raw(q).Count(count).Error
}

func (a *EventLogRepository) QueryWithArgs(q string, args ...interface{}) (*models.EventLog, error) {
	var contracts *models.EventLog
	err := a.database.Raw(q, args...).Find(&contracts).Error
	if err != nil {
		return nil, err
	}
	return contracts, nil
}

func (a *EventLogRepository) QueryRecordsWithArgs(q string, args ...interface{}) ([]*models.EventLog, error) {
	var contracts []*models.EventLog
	err := a.database.Raw(q, args...).Find(&contracts).Error
	if err != nil {
		return nil, err
	}
	return contracts, nil
}

func (a *EventLogRepository) RawSmartSelect(q string, res interface{}, args ...interface{}) error {
	return a.database.Raw(q, args...).Scan(res).Error
}
