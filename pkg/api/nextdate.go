package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DateFormat = "20060102"

func afterNow(date, now time.Time) bool {
	return date.Truncate(24 * time.Hour).After(now.Truncate(24 * time.Hour))
}

func contains(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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

func NextDate(now time.Time, dstart string, repeat string) (string, error) {
	if repeat == "" {
		return "", errors.New("пустое правило повторения")
	}

	start, err := time.Parse(DateFormat, dstart)
	if err != nil {
		return "", errors.New("некорректный формат даты dstart: " + dstart)
	}

	var rule string
	var params string

	// Ищем первую букву (правило)
	for i, r := range repeat {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			rule = string(r)
			params = strings.TrimSpace(repeat[i+1:])
			break
		}
	}

	if rule == "" {
		return "", errors.New("не найдено правило повторения в строке: " + repeat)
	}

	switch rule {
	case "d":
		interval, err := strconv.Atoi(params)
		if err != nil {
			return "", errors.New("некорректный интервал для правила d: не число (" + params + ")")
		}
		if interval <= 0 || interval > 400 {
			return "", errors.New("интервал для правила d должен быть от 1 до 400 (получено: " + params + ")")
		}

		date := start
		date = date.AddDate(0, 0, interval)
		for !afterNow(date, now) {
			date = date.AddDate(0, 0, interval)
		}
		return date.Format(DateFormat), nil

	case "y":
		if params != "" {
			return "", errors.New("для правила y не должно быть дополнительных параметров")
		}

		date := start
		date = addYears(date, 1)
		for !afterNow(date, now) {
			date = addYears(date, 1)
		}
		return date.Format(DateFormat), nil

	case "w":
		if params == "" {
			return "", errors.New("для правила w требуется ровно один параметр (список дней недели через запятую)")
		}
		daysStr := strings.Split(params, ",")
		var days []int
		for _, s := range daysStr {
			day, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", errors.New("день недели должен быть числом: " + s)
			}
			if day < 1 || day > 7 {
				return "", errors.New("день недели должен быть от 1 до 7 (получено: " + s + ")")
			}
			days = append(days, day)
		}

		date := start
		// Ищем ближайший подходящий день недели
		for i := 0; i < 365*2; i++ { // ограничим поиск 2 годами
			date = date.AddDate(0, 0, 1)
			if !afterNow(date, now) {
				continue
			}
			weekday := int(date.Weekday())
			if weekday == 0 {
				weekday = 7 // Воскресенье = 7
			}
			if contains(days, weekday) {
				return date.Format(DateFormat), nil
			}
		}
		return "", errors.New("не удалось найти подходящий день недели")

	case "m":
		if params == "" {
			return "", errors.New("для правила m требуется хотя бы один параметр (список дней месяца)")
		}
		parts := strings.Fields(params)
		if len(parts) == 0 {
			return "", errors.New("неверный формат для правила m")
		}

		daysStr := strings.Split(parts[0], ",")
		var days []int
		for _, s := range daysStr {
			day, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return "", errors.New("день месяца должен быть числом: " + s)
			}
			if day < -2 || day > 31 || day == 0 {
				return "", errors.New("день месяца должен быть от -2 до 31 (кроме 0), получено: " + s)
			}
			days = append(days, day)
		}

		var months []int
		// Если есть второй блок после запятой — это месяцы
		if len(parts) > 1 {
			monthsStr := strings.Split(parts[1], ",")
			for _, s := range monthsStr {
				month, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil {
					return "", errors.New("месяц должен быть числом: " + s)
				}
				if month < 1 || month > 12 {
					return "", errors.New("месяц должен быть от 1 до 12 (получено: " + s + ")")
				}
				months = append(months, month)
			}
		} else {
			// Если месяцы не указаны - используем все месяцы
			for i := 1; i <= 12; i++ {
				months = append(months, i)
			}
		}

		date := start
		// Ищем ближайший подходящий день месяца
		for i := 0; i < 365*2; i++ { // ограничим поиск 2 годами
			date = date.AddDate(0, 0, 1)
			if !afterNow(date, now) {
				continue
			}

			day := date.Day()
			month := int(date.Month())

			validDay := false
			for _, d := range days {
				if d > 0 && d == day {
					validDay = true
					break
				}
				// Последний день месяца
				if d == -1 && date.AddDate(0, 0, 1).Day() == 1 {
					validDay = true
					break
				}
				// Предпоследний день месяца
				if d == -2 && date.AddDate(0, 0, 2).Day() == 1 {
					validDay = true
					break
				}
			}

			if validDay && contains(months, month) {
				return date.Format(DateFormat), nil
			}
		}
		return "", errors.New("не удалось найти подходящую дату для правила m")

	default:
		return "", errors.New("неизвестное правило повторения: " + rule)
	}
}

func nextDateHandler(w http.ResponseWriter, r *http.Request) {
	nowStr := r.FormValue("now")
	dateStr := r.FormValue("date")
	repeat := r.FormValue("repeat")

	var now time.Time
	if nowStr == "" {
		now = time.Now()
	} else {
		var err error
		now, err = time.Parse(DateFormat, nowStr)
		if err != nil {
			http.Error(w, "некорректный формат параметра now", http.StatusBadRequest)
			return
		}
	}
	if dateStr == "" || repeat == "" {
		http.Error(w, "обязательные параметры date и repeat не указаны", http.StatusBadRequest)
		return
	}
	result, err := NextDate(now, dateStr, repeat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Write([]byte(result))
}
