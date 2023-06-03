package repositories

import (
	"database/sql"
	"websockets/models"
)

type Room struct {
	Id      string
	Name    string
	Private bool
}

func (room *Room) GetId() string {
	return room.Id
}

func (room *Room) GetName() string {
	return room.Name
}

func (room *Room) GetPrivate() bool {
	return room.Private
}

type RoomRepository struct {
	Db *sql.DB
}

func (repo *RoomRepository) FindRoomByName(name string) models.Room {
	row := repo.Db.QueryRow("SELECT id, name, private FROM room WHERE name = ? LIMIT 1", name)

	var room Room

	if err := row.Scan(&room.Id, &room.Name, &room.Private); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		panic(err)
	}
	return &room
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
