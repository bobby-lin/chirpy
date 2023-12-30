package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
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
	Users  map[int]User  `json:"users"`
}

type Chirp struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

type User struct {
	ID       int    `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password,omitempty"`
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
	dbStructure, err := db.loadDB()
	nextIndex := len(dbStructure.Chirps) + 1

	newChirp := Chirp{
		ID:   nextIndex,
		Body: body,
	}

	if dbStructure.Chirps == nil {
		dbStructure.Chirps = map[int]Chirp{
			nextIndex: newChirp,
		}
	} else {
		dbStructure.Chirps[nextIndex] = newChirp
	}

	if err != nil {
		return Chirp{}, err
	}

	err = db.writeDB(dbStructure)
	if err != nil {
		return Chirp{}, err
	}

	return newChirp, nil
}

func (db *DB) GetChirps() ([]Chirp, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	chirpList := make([]Chirp, len(dbStructure.Chirps))

	i := 0
	for _, v := range dbStructure.Chirps {
		chirpList[i] = v
		i++
	}

	return chirpList, nil
}

func (db *DB) GetChirp(id int) (Chirp, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		log.Fatal(err)
		return Chirp{}, err
	}

	c, ok := dbStructure.Chirps[id]
	if !ok {
		return Chirp{}, errors.New("chirp does not exist")
	}

	return c, nil
}

func (db *DB) CreateUser(email, password string) (User, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		return User{}, err
	}

	users := dbStructure.Users
	for _, v := range users {
		if v.Email == email {
			return User{}, errors.New(fmt.Sprintf("email already exist: %s", email))
		}
	}

	nextIndex := len(users) + 1

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	u := User{
		ID:       nextIndex,
		Email:    email,
		Password: string(passwordHash),
	}

	if users == nil {
		users = map[int]User{
			nextIndex: u,
		}
	} else {
		users[nextIndex] = u
	}

	err = db.writeDB(dbStructure)
	if err != nil {
		return User{}, err
	}

	u.Password = "" // Don't return password :)

	return u, nil
}

func (db *DB) GetUser(email string) (User, error) {
	dbStructure, err := db.loadDB()
	if err != nil {
		return User{}, err
	}

	userId := -1

	for k, v := range dbStructure.Users {
		if v.Email == email {
			userId = k
			break
		}
	}

	if userId == -1 {
		return User{}, errors.New(fmt.Sprintf("cannot find user with email: %s", email))
	}

	return dbStructure.Users[userId], nil
}

func (db *DB) ensureDB() error {
	_, err := os.ReadFile(db.path)

	if errors.Is(err, fs.ErrNotExist) {
		log.Println("The database.json does not exist! Creating a new file...")
		err = os.WriteFile(db.path, []byte(""), 0666)
		err = db.writeDB(DBStructure{
			Chirps: map[int]Chirp{},
			Users:  map[int]User{},
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

	dbStructure := DBStructure{}

	err = json.Unmarshal(file, &dbStructure)
	if err != nil {
		log.Fatal(err)
		return DBStructure{}, err
	}

	return dbStructure, nil
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