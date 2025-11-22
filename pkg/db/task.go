package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	api "github.com/vita269/final-boss/pkg/dates"
)

type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

const dateFormat = "20060102"

func InitDB(database *sql.DB) error {
	if database == nil {
		return errors.New("база данных не может быть nil")
	}
	scheduler = database
	return nil
}

func AddTask(task *Task) (int64, error) {
	log.Printf("AddTask: inserting date=%s, title=%s, repeat=%s", task.Date, task.Title, task.Repeat)
	if scheduler == nil {
		return 0, errors.New("база данных не инициализирована")
	}

	var id int64

	query := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)`

	res, err := scheduler.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		return 0, err
	}
	id, err = res.LastInsertId()

	return id, err
}

func CheckDate(task *Task) error {
	log.Printf("CheckDate called: date=%s, repeat=%s", task.Date, task.Repeat)
	now := time.Now()

	if task.Date == "" {
		task.Date = now.Format(dateFormat)
		log.Printf("CheckDate result: date=%s (today)", task.Date)
		return nil
	}

	taskTime, err := time.Parse(dateFormat, task.Date)
	if err != nil {
		return errors.New("дата должна быть в формате 20060102")
	}

	if task.Repeat != "" {
		_, err := api.ParseRepeatRule(task.Repeat)
		if err != nil {
			return err
		}
	}

	taskDay := time.Date(taskTime.Year(), taskTime.Month(), taskTime.Day(), 0, 0, 0, 0, taskTime.Location())
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if taskDay.Before(nowDay) {
		if task.Repeat == "" {
			// Для неповторяющихся - переносим на сегодня
			task.Date = now.Format(dateFormat)
			log.Printf("CheckDate result: date=%s (corrected to today)", task.Date)
		} else {
			// Для повторяющихся - вычисляем следующую дату
			nextDate, err := api.CalculateNextDateForTask(time.Now(), task.Date, task.Repeat)
			if err != nil {
				return err
			}
			task.Date = nextDate.Format("20060102")
			log.Printf("CheckDate result: date=%s (repeating task - next date)", task.Date)
		}
		return nil
	}
	return nil
}

func Tasks(limit int, search string) ([]*Task, error) {
	if scheduler == nil {
		return nil, errors.New("база данных не инициализирована")
	}

	var query string
	var args []interface{}

	switch {
	case search == "":
		query = "SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date, id LIMIT ?"
		args = []interface{}{limit}

	case isDate(search):
		dateStr := parseDate(search).Format(dateFormat)
		query = "SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date, id LIMIT ?"
		args = []interface{}{dateStr, limit}

	default:
		searchPattern := "%" + search + "%"
		query = "SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date, id LIMIT ?"
		args = []interface{}{searchPattern, searchPattern, limit}
	}

	return executeQuery(query, args)
}

// Вспомогательные функции для читаемости
func isDate(str string) bool {
	_, err := time.Parse("02.01.2006", str)
	return err == nil
}

func parseDate(str string) time.Time {
	date, _ := time.Parse("02.01.2006", str)
	return date
}

// Вынесенная функция выполнения запроса
func executeQuery(query string, args []interface{}) ([]*Task, error) {
	rows, err := scheduler.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		var id int64
		err := rows.Scan(&id, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			return nil, err
		}
		task.ID = strconv.FormatInt(id, 10)
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if tasks == nil {
		tasks = []*Task{}
	}

	return tasks, nil
}

func GetTask(id string) (*Task, error) {
	if scheduler == nil {
		return nil, errors.New("база данных не инициализирована")
	}
	taskID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("некорректный идентификатор задачи")
	}

	query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?"

	task := &Task{}
	var dbID int64

	err = scheduler.QueryRow(query, taskID).Scan(&dbID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("задача не найдена")
		}
		return nil, err
	}
	task.ID = strconv.FormatInt(dbID, 10)
	return task, nil
}

func UpdateTask(task *Task) error {
	if scheduler == nil {
		return errors.New("база данных не инициализирована")
	}

	taskID, err := strconv.ParseInt(task.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("некорректный идентификатор задачи")
	}

	log.Printf("Updating task ID=%s with data: date=%s, title=%s, comment=%s, repeat=%s",
		task.ID, task.Date, task.Title, task.Comment, task.Repeat)

	query := `UPDATE scheduler SET date=?, title=?, comment=?, repeat=? WHERE id=?`

	res, err := scheduler.Exec(query, task.Date, task.Title, task.Comment, task.Repeat, taskID)
	if err != nil {
		log.Printf("Update error: %v", err)
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		log.Printf("RowsAffected error: %v", err)
		return err
	}
	log.Printf("Rows affected: %d", count)
	if count == 0 {
		return fmt.Errorf("задача не найдена")
	}
	return nil
}

func DeleteTask(id string) error {
	taskID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("некорректный идентификатор задачи")
	}
	query := "DELETE FROM scheduler WHERE id = ?"
	res, err := scheduler.Exec(query, taskID)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("задача не найдена")
	}

	return nil
}

func UpdateDate(id string, newDate string) error {
	if scheduler == nil {
		return errors.New("база данных не инициализирована")
	}

	taskID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("некорректный идентификатор задачи")
	}

	query := "UPDATE scheduler SET date = ? WHERE id = ?"
	res, err := scheduler.Exec(query, newDate, taskID)
	if err != nil {
		return err
	}

	count, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("задача не найдена")
	}

	return nil
}
