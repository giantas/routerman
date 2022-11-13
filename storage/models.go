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
	InitDb                      string `query:"InitDb"`
	CreateUser                  string `query:"CreateUser"`
	GetUserById                 string `query:"GetUserById"`
	GetUsers                    string `query:"GetUsers"`
	GetDeviceById               string `query:"GetDeviceById"`
	GetDevices                  string `query:"GetDevices"`
	GetDevicesByUserId          string `query:"GetDevicesByUserId"`
	UpdateUser                  string `query:"UpdateUser"`
	DeleteUserById              string `query:"DeleteUserById"`
	CreateDevice                string `query:"CreateDevice"`
	UpdateDevice                string `query:"UpdateDevice"`
	DeleteDeviceById            string `query:"DeleteDeviceById"`
	DeleteDeviceByUserId        string `query:"DeleteDeviceByUserId"`
	CreateBandwidthSlot         string `query:"CreateBandwidthSlot"`
	GetBandwidthSlotById        string `query:"GetBandwidthSlotById"`
	GetBandwidthSlots           string `query:"GetBandwidthSlots"`
	GetBandwidthSlotsByUserId   string `query:"GetBandwidthSlotsByUserId"`
	DeleteBandwidthSlotById     string `query:"DeleteBandwidthSlotById"`
	DeleteBandwidthSlotByUserId string `query:"DeleteBandwidthSlotByUserId"`
}](dbScript)

type DbConfig struct {
	Init bool
	URI  string
}

type Store struct {
	UserStore          UserStorage
	DeviceStore        DeviceStorage
	BandwidthSlotStore BandwidthSlotStorage
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		UserStore:          UserStore{db: db},
		DeviceStore:        DeviceStore{db: db},
		BandwidthSlotStore: BandwidthSlotStore{db: db},
	}
}

func ConnectDatabase(cfg DbConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", cfg.URI)
	if err != nil {
		return db, err
	}
	err = db.Ping()
	if err != nil {
		return db, err
	}
	if cfg.Init {
		_, err := db.Exec(Q.InitDb)
		if err != nil {
			return db, err
		}
	}
	return db, err
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
	ReadManyByUserId(userId int, pageSize, pageNumber int) ([]Device, error)
	Update(device Device) error
	Delete(id int) error
	DeleteByUserId(userId int) error
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

func (d DeviceStore) ReadManyByUserId(userId int, pageSize, pageNumber int) ([]Device, error) {
	db := d.db
	devices := make([]Device, 0)
	limit := pageSize
	offset := 0
	if pageNumber > 1 {
		offset = (pageNumber - 1) * pageSize
	}

	rows, err := db.Query(Q.GetDevicesByUserId, userId, limit, offset)
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

func (d DeviceStore) DeleteByUserId(userId int) error {
	db := d.db
	_, err := db.Exec(Q.DeleteDeviceByUserId, userId)
	return err
}

type BandwidthSlot struct {
	Id       int
	UserId   int
	RemoteId int
}

func (slot BandwidthSlot) GetUser(userStore UserStore) (User, error) {
	var user User
	if slot.UserId == 0 {
		return user, fmt.Errorf("no user assigned")
	}
	return userStore.Read(slot.UserId)
}

type BandwidthSlotStorage interface {
	Create(slot *BandwidthSlot) error
	Read(id int) (BandwidthSlot, error)
	ReadMany(pageSize, pageNumber int) ([]BandwidthSlot, error)
	ReadManyByUserId(userId int, pageSize, pageNumber int) ([]BandwidthSlot, error)
	Delete(id int) error
	DeleteByUserId(userId int) error
}

type BandwidthSlotStore struct {
	db *sql.DB
}

func (s BandwidthSlotStore) Create(slot *BandwidthSlot) error {
	db := s.db
	return db.QueryRow(Q.CreateBandwidthSlot, slot.UserId, slot.RemoteId).Scan(&slot.Id)
}

func (s BandwidthSlotStore) Read(id int) (BandwidthSlot, error) {
	db := s.db
	var slot BandwidthSlot
	err := db.QueryRow(Q.GetBandwidthSlotById, id).Scan(&slot.Id, &slot.UserId, &slot.RemoteId)
	if err == sql.ErrNoRows {
		return slot, fmt.Errorf("bandwidth slot not found '%d'", id)
	}
	return slot, err
}

func (s BandwidthSlotStore) ReadMany(pageSize, pageNumber int) ([]BandwidthSlot, error) {
	db := s.db
	slots := make([]BandwidthSlot, 0)
	limit := pageSize
	offset := 0
	if pageNumber > 1 {
		offset = (pageNumber - 1) * pageSize
	}

	rows, err := db.Query(Q.GetBandwidthSlots, limit, offset)
	if err != nil {
		return slots, err
	}
	defer rows.Close()

	for rows.Next() {
		var slot BandwidthSlot
		err := rows.Scan(&slot.Id, &slot.UserId, &slot.RemoteId)
		if err != nil {
			return slots, err
		}
		slots = append(slots, slot)
	}

	return slots, rows.Err()
}

func (s BandwidthSlotStore) ReadManyByUserId(userId int, pageSize, pageNumber int) ([]BandwidthSlot, error) {
	db := s.db
	slots := make([]BandwidthSlot, 0)
	limit := pageSize
	offset := 0
	if pageNumber > 1 {
		offset = (pageNumber - 1) * pageSize
	}

	rows, err := db.Query(Q.GetBandwidthSlotsByUserId, userId, limit, offset)
	if err != nil {
		return slots, err
	}
	defer rows.Close()

	for rows.Next() {
		var slot BandwidthSlot
		err := rows.Scan(&slot.Id, &slot.UserId, &slot.RemoteId)
		if err != nil {
			return slots, err
		}
		slots = append(slots, slot)
	}

	return slots, rows.Err()
}

func (s BandwidthSlotStore) Delete(id int) error {
	db := s.db
	_, err := db.Exec(Q.DeleteBandwidthSlotById, id)
	return err
}
func (s BandwidthSlotStore) DeleteByUserId(userId int) error {
	db := s.db
	_, err := db.Exec(Q.DeleteBandwidthSlotByUserId, userId)
	return err
}
