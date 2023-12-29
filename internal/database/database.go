package database

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"sync"
)

type DB struct {
	path string
	mux  *sync.RWMutex
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
}

type Chirp struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

func NewDB(path string) (*DB, error) {
	db := DB{
		path: path,
		mux:  &sync.RWMutex{},
	}

	err := db.ensureDB()
	if err != nil {
		return nil, err
	}

	return &db, nil
}

func (db *DB) CreateChirp(body string) (Chirp, error) {
	chirps, err := db.loadDB()
	nextIndex := len(chirps.Chirps) + 1

	newChirp := Chirp{
		ID:   nextIndex,
		Body: body,
	}

	if chirps.Chirps == nil {
		chirps.Chirps = map[int]Chirp{
			nextIndex: newChirp,
		}
	} else {
		chirps.Chirps[nextIndex] = newChirp
	}

	if err != nil {
		return Chirp{}, err
	}

	err = db.writeDB(chirps)
	if err != nil {
		return Chirp{}, err
	}

	return newChirp, nil
}

func (db *DB) GetChirps() ([]Chirp, error) {
	chirps, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	chirpList := make([]Chirp, len(chirps.Chirps))

	i := 0
	for _, v := range chirps.Chirps {
		chirpList[i] = v
		i++
	}

	return chirpList, nil
}

func (db *DB) ensureDB() error {
	_, err := os.ReadFile(db.path)

	if errors.Is(err, fs.ErrNotExist) {
		log.Println("The database.json does not exist! Creating a new file...")
		err = os.WriteFile(db.path, []byte(""), 0666)
		err = db.writeDB(DBStructure{
			Chirps: map[int]Chirp{},
		})
	}

	if err != nil {
		return err
	}

	return nil
}

func (db *DB) loadDB() (DBStructure, error) {
	file, err := os.ReadFile(db.path)
	if err != nil {
		log.Fatal(err)
		return DBStructure{}, err
	}

	chirps := DBStructure{}

	err = json.Unmarshal(file, &chirps)
	if err != nil {
		log.Fatal(err)
		return DBStructure{}, err
	}

	return chirps, nil
}

func (db *DB) writeDB(dbStructure DBStructure) error {
	db.mux.Lock()
	defer db.mux.Unlock()

	file, _ := json.MarshalIndent(dbStructure, "", "    ")

	err := os.WriteFile(db.path, file, 0644)
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}
