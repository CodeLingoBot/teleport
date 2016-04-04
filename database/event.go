package database

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"bytes"
	"log"
	"time"
)

type Event struct {
	Id            string  `db:"id"`
	Kind          string  `db:"kind"`
	Status        string  `db:"status"`
	TriggerTag    string  `db:"trigger_tag"`
	TriggerEvent  string  `db:"trigger_event"`
	TransactionId string  `db:"transaction_id"`
	Data          *string `db:"data"`
}

// Every sleepTime interval, create a batch with unbatched events
func (db *Database) BatchEvents(sleepTime time.Duration) {
	for {
		err := db.CreateBatchesFromEvents()

		if err != nil {
			log.Printf("Error creating batch! %v\n", err)
		}

		time.Sleep(sleepTime)
	}
}

func (db *Database) GetEvents(status string) ([]Event, error) {
	var events []Event
	err := db.selectObjs(&events, "SELECT * FROM teleport.event WHERE status = $1 ORDER BY id ASC;", status)
	return events, err
}

func (e *Event) UpdateQuery(tx *sqlx.Tx) {
	tx.MustExec(
		"UPDATE teleport.event SET status = $1, data = $2 WHERE id = $3;",
		e.Status,
		e.Data,
		e.Id,
	)
}

// Implement Stringer
func (e *Event) String() string {
	return fmt.Sprintf(
		"%s,%s,%s,%s,%s,%s",
		e.Id,
		e.Kind,
		e.TriggerTag,
		e.TriggerEvent,
		e.TransactionId,
		*e.Data,
	)
}

// Group all events 'waiting_batch' and create a batch with them.
func (db *Database) CreateBatchesFromEvents() error {
	// Get events waiting replication
	events, err := db.GetEvents("waiting_batch")

	if err != nil {
		return err
	}

	// Stop if there are no events
	if len(events) == 0 {
		return nil
	}

	// Start a transaction
	tx := db.NewTransaction()

	// Store batch data
	var batchBuffer bytes.Buffer

	for _, event := range events {
		// Write event data to batch data
		batchBuffer.WriteString(event.String())
		batchBuffer.WriteString("\n")

		// Update event status to batched
		event.Status = "batched"
		event.UpdateQuery(tx)
	}

	// Allocate a new batch
	batch := NewBatch(batchBuffer.Bytes())

	// Insert batch
	batch.InsertQuery(tx)

	// Commit to database, returning errors
	return tx.Commit()
}
