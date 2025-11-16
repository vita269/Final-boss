package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

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
func NormalizeRepeatWithStep(repeat string) (string, int, error) {
	repeat = strings.TrimSpace(repeat)
	if repeat == "" {
		return "", 0, errors.New("формат повторения пуст")
	}

	// Сначала пробуем парсить с шагом
	parts := strings.Fields(repeat)
	if len(parts) == 2 {
		if step, err := strconv.Atoi(parts[1]); err == nil && step > 0 {
			switch parts[0] {
			case "d":
				return "daily", step, nil
			case "w":
				return "weekly", step, nil
			case "m":
				return "monthly", step, nil
			case "y":
				return "yearly", step, nil
			}
		}
		if parts[0] == "w" {
			// Проверяем что это валидные дни недели (1-7)
			days := strings.Split(parts[1], ",")
			valid := true
			for _, dayStr := range days {
				if day, err := strconv.Atoi(dayStr); err != nil || day < 1 || day > 7 {
					valid = false
					break
				}
			}
			if valid {
				return "weekly_days", 0, nil
			}

		}
	}

	// Если не получилось с шагом, проверяем простые случаи
	switch {
	case repeat == "d" || repeat == "daily":
		return "daily", 1, nil
	//case repeat == "w" || repeat == "weekly":
	//	return "weekly", 1, nil
	case repeat == "m" || repeat == "monthly":
		return "monthly", 1, nil
	case repeat == "y" || repeat == "yearly":
		return "yearly", 1, nil
	default:
		return "", 0, fmt.Errorf("неверный формат повторения: %q", repeat)
	}

}
func CheckDate(task *Task) error {
	log.Printf("CheckDate called: date=%s, repeat=%s", task.Date, task.Repeat)
	now := time.Now()

	if task.Date == "" {
		task.Date = now.Format("20060102")
		log.Printf("CheckDate result: date=%s (today)", task.Date)
		return nil
	}

	taskTime, err := time.Parse("20060102", task.Date)
	if err != nil {
		return errors.New("дата должна быть в формате 20060102")
	}

	if task.Repeat != "" {
		_, _, err := NormalizeRepeatWithStep(task.Repeat)
		if err != nil {
			return err
		}
	}

	taskDay := time.Date(taskTime.Year(), taskTime.Month(), taskTime.Day(), 0, 0, 0, 0, taskTime.Location())
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if taskDay.Before(nowDay) {
		if task.Repeat == "" {
			// Для неповторяющихся - переносим на сегодня
			task.Date = now.Format("20060102")
			log.Printf("CheckDate result: date=%s (corrected to today)", task.Date)
		} else {
			// Для повторяющихся - вычисляем следующую дату
			nextDate, err := CalculateNextDate(task, now)
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

func CalculateNextDate(task *Task, now time.Time) (time.Time, error) {
	log.Printf("CalculateNextDate CALLED: task.Date=%s, task.Repeat=%s, now=%s",
		task.Date, task.Repeat, now.Format("20060102"))

	if task == nil {
		return time.Time{}, errors.New("task не может быть nil")
	}
	if task.Repeat == "" {
		return time.Time{}, errors.New("правило повторения не указано")
	}

	// Нормализуем правило повторения
	repeatType, step, err := NormalizeRepeatWithStep(task.Repeat)
	if err != nil {
		return time.Time{}, err
	}

	// Парсим дату задачи
	start, err := time.Parse("20060102", task.Date)
	if err != nil {
		return time.Time{}, errors.New("некорректный формат даты задачи")
	}

	now = now.Truncate(24 * time.Hour) // Убираем время, оставляем только дату
	next := start

	log.Printf("CalculateNextDate: start=%s, repeatType=%s, step=%d",
		start.Format("20060102"), repeatType, step)

	switch repeatType {
	case "daily":
		// Для daily просто добавляем дни
		next = next.AddDate(0, 0, step)
		for !afterNow(now, next) {
			next = next.AddDate(0, 0, step)
		}
		log.Printf("CalculateNextDate RESULT: %s", next.Format("20060102"))
		return next, nil

	case "weekly":
		// Для weekly добавляем недели
		for !afterNow(now, next) {
			next = next.AddDate(0, 0, 7*step)
		}
		log.Printf("CalculateNextDate RESULT: %s", next.Format("20060102"))
		return next, nil

	case "weekly_days":
		// ДОБАВЛЯЕМ ОБРАБОТКУ ДНЕЙ НЕДЕЛИ
		// "w 1,3,5" означает понедельник, среда, пятница (1=пн, 7=вс)
		daysStr := strings.Split(strings.Split(task.Repeat, " ")[1], ",")
		var days []int
		for _, dayStr := range daysStr {
			day, _ := strconv.Atoi(dayStr)
			days = append(days, day)
		}

		// Ищем следующий подходящий день недели
		for !afterNow(now, next) {
			next = next.AddDate(0, 0, 1) // Добавляем по одному дню

			// Проверяем день недели (1=пн, 7=вс, но в Go: 1=вс, 7=сб)
			weekday := int(next.Weekday())
			if weekday == 0 { // В Go воскресенье = 0
				weekday = 7
			}

			// Проверяем совпадает ли день недели с нужными днями
			for _, targetDay := range days {
				if weekday == targetDay {
					break
				}
			}
		}
		log.Printf("CalculateNextDate RESULT: %s", next.Format("20060102"))
		return next, nil

	case "monthly":
		// Для monthly добавляем месяцы с корректировкой последнего дня
		for !afterNow(now, next) {
			year, month, _ := next.Date()
			nextMonth := time.Date(year, month+time.Month(step), 1, 0, 0, 0, 0, next.Location())
			lastDay := nextMonth.AddDate(0, 0, -1).Day()

			day := next.Day()
			if day > lastDay {
				day = lastDay
			}

			next = time.Date(year, month+time.Month(step), day, next.Hour(), next.Minute(), next.Second(), next.Nanosecond(), next.Location())
		}
		log.Printf("CalculateNextDate RESULT: %s", next.Format("20060102"))
		return next, nil

	case "yearly":
		// Для yearly добавляем годы с корректировкой високосных лет
		next = addYears(next, step)
		for !afterNow(now, next) {
			next = addYears(next, step)

		}
		log.Printf("CalculateNextDate RESULT: %s", next.Format("20060102"))
		return next, nil

	default:
		return time.Time{}, fmt.Errorf("неизвестный тип повторения: %s", repeatType)
	}
}
func addYears(date time.Time, years int) time.Time {
	newDate := date.AddDate(years, 0, 0)

	// Корректировка для 29 февраля
	if date.Month() == time.February && date.Day() == 29 {
		// Если в новом году 29 февраля не существует, переносим на 1 марта
		if newDate.Month() != time.February || newDate.Day() != 29 {
			newDate = time.Date(newDate.Year(), time.March, 1, 0, 0, 0, 0, newDate.Location())
		}
	}

	return newDate
}

// Вспомогательная функция для сравнения дат
func afterNow(now time.Time, t time.Time) bool {
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return tDay.After(nowDay)
}

func Tasks(limit int, search string) ([]*Task, error) {
	if scheduler == nil {
		return nil, errors.New("база данных не инициализирована")
	}

	var query string
	var args []interface{}

	// Если есть поисковый запрос
	if search != "" {
		// Проверяем, является ли search датой в формате dd.mm.yyyy
		if date, err := time.Parse("02.01.2006", search); err == nil {
			// Это дата - ищем по точному совпадению
			dateStr := date.Format("20060102")
			query = "SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date, id LIMIT ?"
			args = []interface{}{dateStr, limit}
		} else {
			// Это текст - ищем в title и comment
			searchPattern := "%" + search + "%"
			query = "SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date, id LIMIT ?"
			args = []interface{}{searchPattern, searchPattern, limit}
		}
	} else {

		query = "SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date, id LIMIT ?"
		args = []interface{}{limit}
	}

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
