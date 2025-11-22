package dates

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

const DateFormat = "20060102"

// RepeatRule представляет правило повторения
type RepeatRule struct {
	Type   string
	Step   int
	Days   []int // для недельных правил (1-7, 1=пн, 7=вс)
	Months []int // для месячных правил (1-12)
}

// ParseRepeatRule парсит строку правила повторения
func ParseRepeatRule(repeat string) (*RepeatRule, error) {
	log.Printf("ParseRepeatRule called with: %q", repeat)
	repeat = strings.TrimSpace(repeat)
	if repeat == "" {
		return nil, errors.New("формат повторения пуст")
	}

	// Пробуем разные форматы парсинга
	if rule, err := parseExtendedFormat(repeat); err == nil {
		log.Printf("parseExtendedFormat SUCCESS: %+v", rule)
		return rule, nil
	} else {
		log.Printf("parseExtendedFormat ERROR: %v", err)
	}

	if rule, err := parseSimpleFormat(repeat); err == nil {
		log.Printf("parseSimpleFormat SUCCESS: %+v", rule)
		return rule, nil
	} else {
		log.Printf("parseSimpleFormat ERROR: %v", err)
	}

	return nil, fmt.Errorf("неверный формат повторения: %q", repeat)
}

// NextDate вычисляет следующую дату на основе правила (для HTTP API)
func NextDate(now time.Time, startDate string, repeat string) (string, error) {
	if repeat == "" {
		return "", errors.New("пустое правило повторения")
	}

	start, err := time.Parse(DateFormat, startDate)
	if err != nil {
		return "", fmt.Errorf("некорректный формат даты: %s", startDate)
	}

	rule, err := ParseRepeatRule(repeat)
	if err != nil {
		return "", err
	}

	next, err := CalculateNextDate(now, start, rule)
	if err != nil {
		return "", err
	}

	return next.Format(DateFormat), nil
}

// CalculateNextDateForTask вычисляет следующую дату для задачи (для внутреннего использования)
func CalculateNextDateForTask(now time.Time, taskDate string, repeat string) (time.Time, error) {
	if repeat == "" {
		return time.Time{}, errors.New("правило повторения не указано")
	}

	start, err := time.Parse(DateFormat, taskDate)
	if err != nil {
		return time.Time{}, errors.New("некорректный формат даты задачи")
	}

	rule, err := ParseRepeatRule(repeat)
	if err != nil {
		return time.Time{}, err
	}

	return CalculateNextDate(now, start, rule)
}

// parseExtendedFormat парсит расширенный формат "d 3", "w 1,3,5" и т.д.
func parseExtendedFormat(repeat string) (*RepeatRule, error) {
	parts := strings.Fields(repeat)
	if len(parts) < 2 {
		return nil, errors.New("неверный формат")
	}

	ruleType := parts[0]
	params := strings.Join(parts[1:], " ")

	switch ruleType {
	case "d":
		if params == "" {
			return nil, errors.New("для правила d должен быть указан интервал")
		}
		step, err := strconv.Atoi(params)
		if err != nil || step <= 0 || step > 400 {
			return nil, errors.New("интервал для правила d должен быть числом от 1 до 400")
		}
		return &RepeatRule{Type: "daily", Step: step}, nil

	case "w":
		// "w 1,3,5" - дни недели
		daysStr := strings.Split(params, ",")
		var days []int
		for _, dayStr := range daysStr {
			day, err := strconv.Atoi(strings.TrimSpace(dayStr))
			if err != nil || day < 1 || day > 7 {
				return nil, errors.New("день недели должен быть числом от 1 до 7")
			}
			days = append(days, day)
		}
		return &RepeatRule{Type: "weekly_days", Days: days}, nil

	case "m":
		// "m 10,20 1,6" - дни месяца и месяцы
		log.Printf("Monthly rule params: %q", params)
		paramParts := strings.Fields(params)
		log.Printf("Monthly paramParts: %v", paramParts)
		if len(paramParts) == 0 {
			return nil, errors.New("неверный формат для правила m")
		}

		// Парсим дни месяца
		daysStr := strings.Split(paramParts[0], ",")
		var days []int
		for _, s := range daysStr {
			day, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || day < -2 || day > 31 || day == 0 {
				return nil, errors.New("день месяца должен быть от -2 до 31 (кроме 0)")
			}
			days = append(days, day)
		}

		// Парсим месяцы (если указаны)
		var months []int
		if len(paramParts) > 1 {
			monthsPart := strings.Join(paramParts[1:], ",")
			monthsStr := strings.Split(monthsPart, ",")
			for _, s := range monthsStr {
				month, err := strconv.Atoi(strings.TrimSpace(s))
				if err != nil || month < 1 || month > 12 {
					return nil, errors.New("месяц должен быть от 1 до 12")
				}
				months = append(months, month)
			}
		} else {
			// Если месяцы не указаны - используем все
			for i := 1; i <= 12; i++ {
				months = append(months, i)
			}
		}

		return &RepeatRule{Type: "monthly", Days: days, Months: months}, nil

	case "y":
		if params != "" {
			return nil, errors.New("для правила y не должно быть дополнительных параметров")
		}
		return &RepeatRule{Type: "yearly", Step: 1}, nil

	default:
		return nil, errors.New("неизвестный тип правила")
	}
}

// parseSimpleFormat парсит простой формат "daily", "monthly" и т.д.
func parseSimpleFormat(repeat string) (*RepeatRule, error) {
	switch repeat {
	case "daily":
		return &RepeatRule{Type: "daily", Step: 1}, nil
	case "m", "monthly":
		return &RepeatRule{Type: "monthly", Step: 1}, nil
	case "y", "yearly":
		return &RepeatRule{Type: "yearly", Step: 1}, nil
	default:
		return nil, errors.New("неизвестный простой формат")
	}
}

// calculateNextDate вычисляет следующую дату на основе правила
func CalculateNextDate(now, start time.Time, rule *RepeatRule) (time.Time, error) {
	now = now.Truncate(24 * time.Hour)
	next := start

	switch rule.Type {
	case "daily":
		return calculateDaily(next, now, rule.Step), nil

	case "weekly_days":
		return calculateWeeklyDays(next, now, rule.Days), nil

	case "monthly":
		return calculateMonthly(next, now, rule.Days, rule.Months), nil

	case "yearly":
		return calculateYearly(next, now, rule.Step), nil

	default:
		return time.Time{}, errors.New("неизвестный тип повторения: " + rule.Type)
	}
}

func calculateDaily(start, now time.Time, step int) time.Time {
	next := start
	next = next.AddDate(0, 0, step)
	for !afterNow(next, now) {
		next = next.AddDate(0, 0, step)
	}
	return next
}

func calculateWeeklyDays(start, now time.Time, days []int) time.Time {
	next := start
	// Ищем ближайший подходящий день недели
	for i := 0; i < 365*2; i++ {
		next = next.AddDate(0, 0, 1)
		if !afterNow(next, now) {
			continue
		}

		weekday := int(next.Weekday())
		if weekday == 0 {
			weekday = 7 // Воскресенье = 7
		}

		if contains(days, weekday) {
			return next
		}
	}
	return next
}

func calculateMonthly(start, now time.Time, days, months []int) time.Time {
	next := start
	for i := 0; i < 365*2; i++ {
		next = next.AddDate(0, 0, 1)
		if !afterNow(next, now) {
			continue
		}

		day := next.Day()
		month := int(next.Month())

		validDay := false
		for _, d := range days {
			if d > 0 && d == day {
				validDay = true
				break
			}
			// Последний день месяца
			if d == -1 && next.AddDate(0, 0, 1).Day() == 1 {
				validDay = true
				break
			}
			// Предпоследний день месяца
			if d == -2 && next.AddDate(0, 0, 2).Day() == 1 {
				validDay = true
				break
			}
		}

		if validDay && contains(months, month) {
			return next
		}
	}
	return next
}

func calculateYearly(start, now time.Time, step int) time.Time {
	next := start
	next = addYears(next, step)
	for !afterNow(next, now) {
		next = addYears(next, step)
	}
	return next
}

// Вспомогательные функции
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
		if newDate.Month() != time.February || newDate.Day() != 29 {
			newDate = time.Date(newDate.Year(), time.March, 1, 0, 0, 0, 0, newDate.Location())
		}
	}
	return newDate
}
