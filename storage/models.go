package storage

import (
	"database/sql"
	"fmt"

	_ "embed"

	"github.com/midir99/sqload"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed db.sql
var dbScript string

var Q = sqload.MustLoadFromString[struct {
	InitDb           string `query:"InitDb"`
	CreateUser       string `query:"CreateUser"`
	GetUserById      string `query:"GetUserById"`
	GetUsers         string `query:"GetUsers"`
	GetDeviceById    string `query:"GetDeviceById"`
	GetDevices       string `query:"GetDevices"`
	UpdateUser       string `query:"UpdateUser"`
	DeleteUserById   string `query:"DeleteUserById"`
	CreateDevice     string `query:"CreateDevice"`
	UpdateDevice     string `query:"UpdateDevice"`
	DeleteDeviceById string `query:"DeleteDeviceById"`
}](dbScript)

type DbConfig struct {
	Init bool
	URI  string
}

type Store struct {
	UserStore   UserStorage
	DeviceStore DeviceStorage
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		UserStore:   UserStore{db: db},
		DeviceStore: DeviceStore{db: db},
	}
}

func ConnectDatabase(cfg DbConfig) (*Store, error) {
	var store *Store
	db, err := sql.Open("sqlite3", cfg.URI)
	if err != nil {
		return store, err
	}
	err = db.Ping()
	if err != nil {
		return store, err
	}
	if cfg.Init {
		_, err := db.Exec(Q.InitDb)
		if err != nil {
			return store, err
		}
	}
	store = NewStore(db)
	return store, err
}

type User struct {
	Id   int
	Name string
}

type UserStorage interface {
	Create(user *User) error
	Read(id int) (User, error)
	ReadMany(pageSize, pageNumber int) ([]User, error)
	Update(user User) error
	Delete(id int) error
}

type UserStore struct {
	db *sql.DB
}

func (u UserStore) Create(user *User) error {
	db := u.db
	return db.QueryRow(Q.CreateUser, user.Name).Scan(&user.Id)
}

func (u UserStore) Read(id int) (User, error) {
	db := u.db
	var user User
	err := db.QueryRow(Q.GetUserById, id).Scan(&user.Id, &user.Name)
	if err == sql.ErrNoRows {
		return user, fmt.Errorf("user not found '%d'", id)
	}
	return user, err
}

func (u UserStore) ReadMany(pageSize, pageNumber int) ([]User, error) {
	db := u.db
	users := make([]User, 0)
	limit := pageSize
	offset := 0
	if pageNumber > 1 {
		offset = (pageNumber - 1) * pageSize
	}

	rows, err := db.Query(Q.GetUsers, limit, offset)
	if err != nil {
		return users, err
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		err := rows.Scan(&user.Id, &user.Name)
		if err != nil {
			return users, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (u UserStore) Update(user User) error {
	db := u.db
	_, err := db.Exec(Q.UpdateUser, user.Id, user.Name)
	return err
}

func (u UserStore) Delete(id int) error {
	db := u.db
	_, err := db.Exec(Q.DeleteUserById, id)
	return err
}

type Device struct {
	Id     int
	UserId int
	Alias  string
	Mac    string
}

func (device Device) GetUser(userStore UserStore) (User, error) {
	var user User
	if device.UserId == 0 {
		return user, fmt.Errorf("no user assigned")
	}
	return userStore.Read(device.UserId)
}

type DeviceStorage interface {
	Create(device *Device) error
	Read(id int) (Device, error)
	ReadMany(pageSize, pageNumber int) ([]Device, error)
	Update(device Device) error
	Delete(id int) error
}

type DeviceStore struct {
	db *sql.DB
}

func (d DeviceStore) Create(device *Device) error {
	db := d.db
	return db.QueryRow(
		Q.CreateDevice, device.UserId, device.Alias, device.Mac,
	).Scan(&device.Id)
}

func (d DeviceStore) Read(id int) (Device, error) {
	db := d.db
	var device Device
	err := db.QueryRow(Q.GetDeviceById, id).Scan(&device.Id, &device.UserId, &device.Alias, &device.Mac)
	if err == sql.ErrNoRows {
		return device, fmt.Errorf("device not found '%d'", id)
	}
	return device, err
}

func (d DeviceStore) ReadMany(pageSize, pageNumber int) ([]Device, error) {
	db := d.db
	devices := make([]Device, 0)
	limit := pageSize
	offset := 0
	if pageNumber > 1 {
		offset = (pageNumber - 1) * pageSize
	}

	rows, err := db.Query(Q.GetDevices, limit, offset)
	if err != nil {
		return devices, err
	}
	defer rows.Close()

	for rows.Next() {
		var device Device
		err := rows.Scan(&device.Id, &device.UserId, &device.Alias, &device.Mac)
		if err != nil {
			return devices, err
		}
		devices = append(devices, device)
	}

	return devices, rows.Err()
}

func (d DeviceStore) Update(device Device) error {
	db := d.db
	_, err := db.Exec(
		Q.UpdateDevice, device.Id, device.UserId, device.Alias, device.Mac,
	)
	return err
}

func (d DeviceStore) Delete(id int) error {
	db := d.db
	_, err := db.Exec(Q.DeleteDeviceById, id)
	return err
}
