package jobs

import (
	"encoding/json"

	"github.com/eqlabs/flow-wallet-api/datastore"
	"github.com/eqlabs/flow-wallet-api/message_client"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GormStore struct {
	db            *gorm.DB
	messageClient *message_client.AmqpClient
}

func NewGormStore(db *gorm.DB, messageClient *message_client.AmqpClient) *GormStore {
	db.AutoMigrate(&Job{})
	return &GormStore{db, messageClient}
}

func (s *GormStore) Jobs(o datastore.ListOptions) (jj []Job, err error) {
	err = s.db.
		Order("created_at desc").
		Limit(o.Limit).
		Offset(o.Offset).
		Find(&jj).Error
	return
}

func (s *GormStore) Job(id uuid.UUID) (j Job, err error) {
	err = s.db.First(&j, "id = ?", id).Error
	return
}

func (s *GormStore) InsertJob(j *Job) error {
	return s.db.Create(j).Error
}

func (s *GormStore) UpdateJob(j *Job) error {
	err := s.db.Save(j).Error
	if err == nil && s.messageClient != nil {
		b, _ := json.Marshal(j)
		s.messageClient.Publish(b, "FlowJobStatusChange", "hivehamlet_event_bus", "direct")
	}
	return err
}
