package db

import (
	"database/sql"
	"github.com/golang/protobuf/ptypes"
	_ "github.com/mattn/go-sqlite3"
	"strings"
	"sync"
	"time"
	"sort"

	"git.neds.sh/matty/entain/racing/proto/racing"
)

// RacesRepo provides repository access to races.
type RacesRepo interface {
	// Init will initialise our races repository.
	Init() error

	// List will return a list of races.
	List(filter *racing.ListRacesRequestFilter) ([]*racing.Race, error)
	// Get will return a race by its id.
	Get(id int64) (*racing.Race, error)
}

type racesRepo struct {
	db   *sql.DB
	init sync.Once
}

// NewRacesRepo creates a new races repository.
func NewRacesRepo(db *sql.DB) RacesRepo {
	return &racesRepo{db: db}
}

// Init prepares the race repository dummy data.
func (r *racesRepo) Init() error {
	var err error

	r.init.Do(func() {
		// For test/example purposes, we seed the DB with some dummy races.
		err = r.seed()
	})

	return err
}

func (r *racesRepo) List(filter *racing.ListRacesRequestFilter) ([]*racing.Race, error) {
	var (
		err   error
		query string
		args  []interface{}
	)

	query = getRaceQueries()[racesList]

	query, args = r.applyFilter(query, filter)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	return r.scanRaces(rows, filter.OrderByAsc)
}

func (r *racesRepo) Get(id int64) (*racing.Race, error) {
	var row = r.db.QueryRow(getRaceQueries()[racesList] + " WHERE id = ?", id)
	return r.scanRace(row);
}

func (r *racesRepo) applyFilter(query string, filter *racing.ListRacesRequestFilter) (string, []interface{}) {
	var (
		clauses []string
		args    []interface{}
	)

	if filter == nil {
		return query, args
	}

	if len(filter.MeetingIds) > 0 {
		clauses = append(clauses, "meeting_id IN ("+strings.Repeat("?,", len(filter.MeetingIds)-1)+"?)")

		for _, meetingID := range filter.MeetingIds {
			args = append(args, meetingID)
		}
	}

	if len(filter.Visible) > 0 {
		clauses = append(clauses, "visible IN ("+strings.Repeat("?,", len(filter.Visible)-1)+"?)")

		for _, visible := range filter.Visible {
			args = append(args, visible)
		}
	}

	if len(clauses) != 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	return query, args
}

func (m *racesRepo) scanRace(row *sql.Row) (*racing.Race, error) {
	var (
		race 			racing.Race
		advertisedStart time.Time
		err 			error
	)
	
	if err := row.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	ts, err := ptypes.TimestampProto(advertisedStart)
	if err != nil {
		return nil, nil
	}

	race.AdvertisedStartTime = ts
	if (advertisedStart.Before(time.Now())) {
		race.Status = "CLOSED"
	} else {
		race.Status = "OPEN"
	}

	return &race, nil
}

func (m *racesRepo) scanRaces(rows *sql.Rows, orderByAsc bool) ([]*racing.Race, error) {
	var races []*racing.Race
	
	for rows.Next() {
		var race racing.Race
		var advertisedStart time.Time

		if err := rows.Scan(&race.Id, &race.MeetingId, &race.Name, &race.Number, &race.Visible, &advertisedStart); err != nil {
			if err == sql.ErrNoRows {
				return nil, nil
			}

			return nil, err
		}

		ts, err := ptypes.TimestampProto(advertisedStart)
		if err != nil {
			return nil, err
		}

		race.AdvertisedStartTime = ts
		if (advertisedStart.Before(time.Now())) {
			race.Status = "CLOSED"
		} else {
			race.Status = "OPEN"
		}

		races = append(races, &race)
	}

	sort.SliceStable(races, func(i, j int) bool {
		if orderByAsc == true {
			return races[i].AdvertisedStartTime.AsTime().Before(races[j].AdvertisedStartTime.AsTime())
		} else {
			return races[i].AdvertisedStartTime.AsTime().After(races[j].AdvertisedStartTime.AsTime())
		}
	})
	return races, nil
}
